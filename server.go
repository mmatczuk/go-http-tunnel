package tunnel

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/mmatczuk/tunnel/id"
	"github.com/mmatczuk/tunnel/log"
	"github.com/mmatczuk/tunnel/proto"
)

// ServerConfig defines configuration for the Server.
type ServerConfig struct {
	// Addr is tcp address to listen on for client connections, ":0" if empty.
	Addr string
	// TLSConfig specifies the tls configuration to use with tls.Listener.
	TLSConfig *tls.Config
	// Listener specifies optional listener that clients would connect to.
	// If Listener is nil tls.Listen("tcp", Addr, TLSConfig) is used.
	Listener net.Listener
	// Logger is optional logger. If nil no logs will be printed.
	Logger log.Logger
}

// Server is responsible for proxying public connections to the client over a
// tunnel connection.
type Server struct {
	*registry
	config     *ServerConfig
	listener   net.Listener
	connPool   *connPool
	httpClient *http.Client
	logger     log.Logger
}

// NewServer creates a new Server.
func NewServer(config *ServerConfig) (*Server, error) {
	listener, err := listener(config)
	if err != nil {
		return nil, fmt.Errorf("tls listener failed :%s", err)
	}

	t := &http2.Transport{}
	pool := newConnPool(t)
	t.ConnPool = pool

	logger := config.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &Server{
		registry:   newRegistry(),
		config:     config,
		listener:   listener,
		connPool:   pool,
		httpClient: &http.Client{Transport: t},
		logger:     logger,
	}, nil
}

func listener(config *ServerConfig) (net.Listener, error) {
	if config.Listener != nil {
		return config.Listener, nil
	}

	if config.Addr == "" {
		panic("Missing Addr")
	}
	if config.TLSConfig == nil {
		panic("Missing TLSConfig")
	}

	return tls.Listen("tcp", config.Addr, config.TLSConfig)
}

// Start starts accepting connections form clients and allowed clients listeners.
// For accepting http traffic one must run server as a handler to http server.
func (s *Server) Start() {
	s.logger.Log(
		"level", 1,
		"action", "start",
		"addr", s.listener.Addr(),
	)
	go s.listenControl()
}

func (s *Server) listenControl() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.logger.Log(
				"level", 2,
				"msg", "accept control connection failed",
				"err", err,
			)
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			continue
		}

		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	logger := log.NewContext(s.logger).With("addr", conn.RemoteAddr())

	logger.Log(
		"level", 1,
		"action", "try connect",
	)

	var (
		identifier id.ID
		req        *http.Request
		resp       *http.Response
		tunnels    map[string]*proto.Tunnel
		err        error
		ok         bool
	)

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		logger.Log(
			"level", 0,
			"msg", "invalid connection type",
			"err", fmt.Errorf("expected tls conn, got %T", conn),
		)
		goto reject
	}

	identifier, err = id.PeerID(tlsConn)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "certificate error",
			"err", err,
		)
		goto reject
	}

	logger = logger.With("identifier", identifier)

	if !s.IsSubscribed(identifier) {
		logger.Log(
			"level", 2,
			"msg", "unknown client",
		)
		goto reject
	}

	if err = conn.SetDeadline(time.Time{}); err != nil {
		logger.Log(
			"level", 2,
			"msg", "setting infinite deadline failed",
			"err", err,
		)
		goto reject
	}

	if err := s.connPool.AddConn(conn, identifier); err != nil {
		logger.Log(
			"level", 2,
			"msg", "adding connection failed",
			"err", err,
		)
		goto reject
	}

	req, err = http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), nil)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake request creation failed",
			"err", err,
		)
		goto reject
	}

	resp, err = s.httpClient.Do(req)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}
	if resp.StatusCode != http.StatusOK {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", fmt.Errorf("Status %s", resp.Status),
		)
		goto reject
	}
	if resp.ContentLength > 0 {
		var done chan struct{}
		go func() {
			err = json.NewDecoder(&io.LimitedReader{
				R: resp.Body,
				N: 126976,
			}).Decode(&tunnels)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Minute):
			err = fmt.Errorf("timeout")
		}

		if err != nil {
			logger.Log(
				"level", 2,
				"msg", "handshake failed",
				"err", err,
			)
			goto reject
		}
		if err = s.AddTunnels(tunnels, identifier); err != nil {
			logger.Log(
				"level", 2,
				"msg", "handshake failed",
				"err", err,
			)
			goto reject
		}
	}

	logger.Log(
		"level", 1,
		"action", "connected",
	)

	return

reject:
	s.logger.Log(
		"level", 1,
		"action", "rejected",
		"addr", conn.RemoteAddr(),
	)

	s.connPool.DeleteConn(identifier)
}

// AddTunnels invokes AddHost or AddListener based on data from proto.Tunnel. If
// a tunnel cannot be added whole batch is reverted.
func (s *Server) AddTunnels(tunnels map[string]*proto.Tunnel, identifier id.ID) error {
	var (
		hosts     []string
		listeners []net.Listener
		err       error
	)

	for name, t := range tunnels {
		switch t.Protocol {
		case proto.HTTP:
			err = s.AddHost(t.Host, identifier)
			if err != nil {
				goto rollback
			}
			hosts = append(hosts, t.Host)
		case proto.TCP, proto.TCP4, proto.TCP6, proto.UNIX:
			var l net.Listener
			l, err = net.Listen(t.Protocol, t.Addr)
			if err != nil {
				goto rollback
			}
			listeners = append(listeners, l)

			err := s.AddListener(l, identifier)
			if err != nil {
				goto rollback
			}
		default:
			err = fmt.Errorf("unsupported protocol for tunnel %s: %s", name, t.Protocol)
			goto rollback
		}
	}

	return nil

rollback:
	for _, h := range hosts {
		s.DeleteHost(h, identifier)
	}

	for _, l := range listeners {
		l.Close()
		s.DeleteListener(l, identifier)
	}

	return err
}

// Unsubscribe removes client from registy, disconnects client if already
// connected and returns it's RegistryItem.
func (s *Server) Unsubscribe(identifier id.ID) *RegistryItem {
	s.connPool.DeleteConn(identifier)
	return s.registry.Unsubscribe(identifier)
}

// AddListener adds listener to client.
func (s *Server) AddListener(l net.Listener, identifier id.ID) error {
	if err := s.registry.AddListener(l, identifier); err != nil {
		return err
	}

	go s.listen(l, identifier)

	return nil
}

func (s *Server) listen(l net.Listener, identifier id.ID) {
	for {
		conn, err := l.Accept()
		if err != nil {
			s.logger.Log(
				"level", 2,
				"msg", "accept connection failed",
				"err", err,
			)
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			continue
		}

		msg := &proto.ControlMessage{
			Action:       proto.Proxy,
			Protocol:     l.Addr().Network(),
			ForwardedFor: conn.RemoteAddr().String(),
			ForwardedBy:  l.Addr().String(),
		}
		go s.proxyConn(identifier, conn, msg)
	}
}

func (s *Server) proxyConn(identifier id.ID, conn net.Conn, msg *proto.ControlMessage) {
	s.logger.Log(
		"level", 2,
		"action", "proxy",
		"ctrlMsg", msg,
	)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := s.proxyRequest(identifier, msg, pr)
	if err != nil {
		s.logger.Log(
			"level", 0,
			"msg", "proxy error",
			"ctrlMsg", msg,
			"err", err,
		)
		conn.Close()
		return
	}

	go transfer(pw, conn, log.NewContext(s.logger).With(
		"dir", "user to client",
		"dst", identifier,
		"src", conn.RemoteAddr(),
	))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Log(
			"level", 0,
			"msg", "proxy error",
			"ctrlMsg", msg,
			"err", err,
		)
		conn.Close()
		return
	}

	transfer(conn, resp.Body, log.NewContext(s.logger).With(
		"dir", "client to user",
		"dst", conn.RemoteAddr(),
		"src", identifier,
	))
}

// ServeHTTP proxies http connection to the client.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := s.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		transfer(w, resp.Body, log.NewContext(s.logger).With(
			"dir", "client to user",
			"dst", r.RemoteAddr,
			"src", r.Host,
		))
	}
}

// RoundTrip is http.RoundTriper implementation.
func (s *Server) RoundTrip(r *http.Request) (*http.Response, error) {
	msg := &proto.ControlMessage{
		Action:       proto.Proxy,
		Protocol:     proto.HTTP,
		ForwardedFor: r.RemoteAddr,
		ForwardedBy:  r.Host,
	}

	identifier, ok := s.Subscriber(r.Host)
	if !ok {
		return nil, fmt.Errorf("proxy request error: %s", errClientNotSubscribed)
	}

	return s.proxyHTTP(identifier, r, msg)
}

func (s *Server) proxyHTTP(identifier id.ID, r *http.Request, msg *proto.ControlMessage) (*http.Response, error) {
	s.logger.Log(
		"level", 2,
		"action", "proxy",
		"ctrlMsg", msg,
	)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := s.proxyRequest(identifier, msg, pr)
	if err != nil {
		return nil, fmt.Errorf("proxy request error: %s", err)
	}

	go func() {
		cw := &countWriter{pw, 0}
		err := r.Write(cw)
		if err != nil {
			s.logger.Log(
				"level", 0,
				"msg", "proxy error",
				"ctrlMsg", msg,
				"err", err,
			)
		}

		s.logger.Log(
			"level", 3,
			"action", "transferred",
			"bytes", cw.count,
			"dir", "user to client",
			"dst", r.Host,
			"src", r.RemoteAddr,
		)

		if r.Body != nil {
			r.Body.Close()
		}
	}()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy error: %s", err)
	}

	return resp, nil
}

func (s *Server) proxyRequest(identifier id.ID, msg *proto.ControlMessage, r io.Reader) (*http.Request, error) {
	if msg.Action != proto.Proxy {
		panic("Invalid action")
	}

	req, err := http.NewRequest(http.MethodPut, s.connPool.URL(identifier), r)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %s", err)
	}
	msg.Update(req.Header)

	return req, nil
}

// Addr returns network address clients connect to.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Stop closes the server.
func (s *Server) Stop() {
	s.logger.Log(
		"level", 1,
		"action", "stop",
	)

	if s.listener != nil {
		s.listener.Close()
	}
}
