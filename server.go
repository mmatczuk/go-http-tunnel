package tunnel

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/andrew-d/id"
	"github.com/mmatczuk/tunnel/log"
	"github.com/mmatczuk/tunnel/proto"
)

// AllowedClient specifies client entry points on server.
type AllowedClient struct {
	// ID is client TLS certificate ID.
	ID id.ID
	// Host is URL host name, http requests to that host will be routed to the client.
	Host string
	// Listeners is a list of listeners, connections the listeners accept
	// will be routed to the client.
	Listeners []net.Listener
}

// ServerConfig defines configuration for the Server.
type ServerConfig struct {
	// Addr is tcp address to listen on for client connections, ":0" if empty.
	Addr string
	// TLSConfig specifies the tls configuration to use with tls.Listener.
	TLSConfig *tls.Config
	// Listener specifies optional listener that clients would connect to.
	// If Listener is nil tls.Listen("tcp", Addr, TLSConfig) is used.
	Listener net.Listener
	// AllowedClients specifies clients that can connect to the server.
	AllowedClients []*AllowedClient
	// Logger is optional logger. If nil no logs will be printed.
	Logger log.Logger
}

// Server is responsible for proxying public connections to the client over a
// tunnel connection.
type Server struct {
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
	s.listenClientListeners()
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
		id     id.ID
		client *AllowedClient
		req    *http.Request
		resp   *http.Response
		err    error
		ok     bool
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

	id, err = peerID(tlsConn)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "certificate error",
			"err", err,
		)
		goto reject
	}

	logger = logger.With("id", id)

	client, ok = s.checkID(id)
	if !ok {
		logger.Log(
			"level", 2,
			"msg", "unknown certificate",
		)
		goto reject
	}

	req, err = http.NewRequest(http.MethodConnect, clientURL(client.Host), nil)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake request creation failed",
			"err", err,
		)
		goto reject
	}

	if err = conn.SetDeadline(time.Time{}); err != nil {
		logger.Log(
			"level", 2,
			"msg", "setting infinite deadline failed",
			"err", err,
		)
		// recoverable
	}

	if err := s.connPool.addHostConn(client.Host, conn); err != nil {
		logger.Log(
			"level", 2,
			"msg", "adding host failed",
			"host", client.Host,
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

	conn.Close()
	if client != nil {
		s.connPool.markHostDead(client.Host)
	}
}

func (s *Server) checkID(id id.ID) (*AllowedClient, bool) {
	for _, c := range s.config.AllowedClients {
		if id.Equals(c.ID) {
			return c, true
		}
	}
	return nil, false
}

func (s *Server) listenClientListeners() {
	for _, client := range s.config.AllowedClients {
		if client.Listeners == nil {
			continue
		}

		for _, l := range client.Listeners {
			go s.listen(l, client)
		}
	}
}

func (s *Server) listen(l net.Listener, client *AllowedClient) {
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
		go s.proxyConn(client.Host, conn, msg)
	}
}

func (s *Server) proxyConn(host string, conn net.Conn, msg *proto.ControlMessage) {
	s.logger.Log(
		"level", 2,
		"action", "proxy",
		"ctrlMsg", msg,
	)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req := clientRequest(host, msg, pr)

	go transfer(pw, conn, log.NewContext(s.logger).With(
		"dir", "user to client",
		"dst", host,
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
		"src", host,
	))
}

// ServeHTTP proxies http connection to the client.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := s.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
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
	return s.proxyHTTP(trimPort(r.Host), r, msg)
}

func (s *Server) proxyHTTP(host string, r *http.Request, msg *proto.ControlMessage) (*http.Response, error) {
	s.logger.Log(
		"level", 2,
		"action", "proxy",
		"ctrlMsg", msg,
	)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req := clientRequest(host, msg, pr)

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
		return nil, fmt.Errorf("proxy request error: %s", err)
	}

	return resp, nil
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
