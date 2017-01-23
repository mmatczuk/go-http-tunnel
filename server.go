package tunnel

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/andrew-d/id"
	"github.com/koding/logging"
	"github.com/mmatczuk/tunnel/proto"
	"golang.org/x/net/http2"
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
	// Log specifies the logger. If nil a default logging.Logger is used.
	Log logging.Logger
}

// Server is responsible for proxying public connections to the client over a
// tunnel connection.
type Server struct {
	config     *ServerConfig
	listener   net.Listener
	connPool   *connPool
	httpClient *http.Client
	log        logging.Logger
}

// NewServer creates a new Server.
func NewServer(config *ServerConfig) (*Server, error) {
	l, err := listener(config)
	if err != nil {
		return nil, fmt.Errorf("tls listener failed :%s", err)
	}

	t := &http2.Transport{}
	p := newConnPool(t)
	t.ConnPool = p

	log := logging.NewLogger("server")
	if config.Log != nil {
		log = config.Log
	}

	return &Server{
		config:     config,
		listener:   l,
		connPool:   p,
		httpClient: &http.Client{Transport: t},
		log:        log,
	}, nil
}

func listener(config *ServerConfig) (net.Listener, error) {
	if config.Listener != nil {
		return config.Listener, nil
	}

	addr := ":0"
	if config.Addr != "" {
		addr = config.Addr
	}

	return tls.Listen("tcp", addr, config.TLSConfig)
}

// Start starts accepting connections form clients and allowed clients listeners.
// For accepting http traffic one must run server as a handler to http server.
func (s *Server) Start() {
	go s.listenControl()
	s.listenClientListeners()
}

func (s *Server) listenControl() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.log.Warning("Accept %s control connection to %q failed: %s",
				s.listener.Addr().Network(), s.listener.Addr().String(), err)
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			continue
		}
		s.log.Info("Accepted %s control connection from %q to %q",
			s.listener.Addr().Network(), conn.RemoteAddr(), s.listener.Addr().String())
		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	var (
		client *AllowedClient
		req    *http.Request
		resp   *http.Response
		err    error
		ok     bool
	)

	id, err := peerID(conn.(*tls.Conn))
	if err != nil {
		s.log.Warning("Certificate error: %s", err)
		goto reject
	}

	client, ok = s.checkID(id)
	if !ok {
		s.log.Warning("Unknown certificate: %q", id.String())
		goto reject
	}

	req, err = http.NewRequest(http.MethodConnect, clientURL(client.Host), nil)
	if err != nil {
		s.log.Error("Invalid host %q for client %q", client.Host, client.ID)
		goto reject
	}

	if err = conn.SetDeadline(time.Time{}); err != nil {
		s.log.Warning("Setting no deadline failed: %s", err)
		// recoverable
	}

	if err := s.connPool.addHostConn(client.Host, conn); err != nil {
		s.log.Warning("Could not add host: %s", err)
		goto reject
	}

	resp, err = s.httpClient.Do(req)
	if err != nil {
		s.log.Warning("Handshake failed %s", err)
		goto reject
	}
	if resp.StatusCode != http.StatusOK {
		s.log.Warning("Handshake failed")
		goto reject
	}

	s.log.Info("Connected to client %s (%q)", conn.RemoteAddr(), client.ID)

	return

reject:
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
			s.log.Warning("Accept %s connection to %q failed: %s",
				s.listener.Addr().Network(), s.listener.Addr().String(), err)
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			continue
		}
		s.log.Debug("Accepted %s connection from %q to %q",
			l.Addr().Network(), conn.RemoteAddr(), l.Addr().String())

		msg := &proto.ControlMessage{
			Action:       proto.Proxy,
			Protocol:     l.Addr().Network(),
			ForwardedFor: conn.RemoteAddr().String(),
			ForwardedBy:  l.Addr().String(),
		}

		go func() {
			err := s.proxyConn(client.Host, conn, msg)
			if err != nil {
				s.log.Warning("Error %s: %s", msg, err)
			}
		}()
	}
}

func (s *Server) proxyConn(host string, conn net.Conn, msg *proto.ControlMessage) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req := proxyRequest(host, msg, pr)

	go transfer("local to remote", pw, conn)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxy request error: %s", err)
	}

	transfer("remote to local", conn, resp.Body)

	return nil
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
		transfer("remote to local", w, resp.Body)
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
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req := proxyRequest(host, msg, pr)

	go func() {
		cw := &countWriter{pw, 0}
		err := r.Write(cw)
		if err != nil {
			s.log.Error("Write to pipe failed: %s", err)
		}
		TransferLog.Debug("Coppied %d bytes from %s", cw.count, "local to remote")
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

// Close closes the server.
func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}

	return s.listener.Close()
}
