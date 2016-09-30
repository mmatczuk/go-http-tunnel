package h2tun

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/andrew-d/id"
	"github.com/koding/h2tun/proto"
	"github.com/koding/logging"
	"golang.org/x/net/http2"
)

// TODO document
//
// TODO (phase2) use sync.Pool to avoid allocations of control message, analyse allocations
// TODO (phase2) add support for UDP and IP by adding `Conns []net.Conn` to AllowedClient
// TODO (phase2) dynamic AllowedClient management
// TODO (phase2) ping, like https://godoc.org/github.com/hashicorp/yamux#Session.Ping
// TODO (phase2) stream compression Accept-Encoding <-> Content-Encoding
// TODO (phase2) add monitoring hooks
// TODO (phase2) add control message stringer

type AllowedClient struct {
	ID        id.ID
	Host      string
	Listeners []net.Listener
}

// ServerConfig is Server configuration object.
type ServerConfig struct {
	// Addr is TCP address to listen on for client connections, ":0" if empty.
	Addr string
	// TLSConfig specifies the TLS configuration to use with tls.Listener.
	TLSConfig *tls.Config
	// Listener is an optional client server connection middleware.
	Listener func(net.Listener) net.Listener
	// AllowedClients specifies clients that can connect to the server.
	AllowedClients []*AllowedClient
	// Log specifies the logger. If nil a default logging.Logger is used.
	Log logging.Logger
}

// Server is a tunnel server.
type Server struct {
	config *ServerConfig

	listener   net.Listener
	connPool   *connPool
	httpClient *http.Client

	log logging.Logger
}

// NewServer creates new Server base on configuration.
func NewServer(config *ServerConfig) (*Server, error) {
	addr := ":0"
	if config.Addr != "" {
		addr = config.Addr
	}

	l, err := tls.Listen("tcp", addr, config.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("tls listener failed :%s", err)
	}
	if config.Listener != nil {
		l = config.Listener(l)
		if l == nil {
			return nil, fmt.Errorf("listener function did not return a listener")
		}
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

	req, err = http.NewRequest(http.MethodConnect, url(client.Host), nil)
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

	s.log.Info("Client %q connected from %q", client.ID, conn.RemoteAddr().String())

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
			Action:       proto.RequestClientSession,
			Protocol:     l.Addr().Network(),
			ForwardedFor: conn.RemoteAddr().String(),
			ForwardedBy:  conn.LocalAddr().String(),
		}

		go func() {
			err := s.proxyConn(client.Host, conn, msg)
			if err != nil {
				s.log.Warning("Error %s: %s", msg, err)
			}
		}()
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	msg := &proto.ControlMessage{
		Action:       proto.RequestClientSession,
		Protocol:     proto.HTTPProtocol,
		ForwardedFor: r.RemoteAddr,
		ForwardedBy:  r.Host,
		URLPath:      r.URL.Path,
	}

	err := s.proxyHTTP(trimPort(r.Host), w, r, msg)
	if err != nil {
		s.log.Warning("Error %s: %s", msg, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func trimPort(hostPort string) (host string) {
	host, _, _ = net.SplitHostPort(hostPort)
	if host == "" {
		host = hostPort
	}
	return
}

func (s *Server) proxyHTTP(host string, w http.ResponseWriter, r *http.Request, msg *proto.ControlMessage) error {
	s.log.Debug("Start %s", msg)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := http.NewRequest(http.MethodPut, url(host), pr)
	if err != nil {
		return fmt.Errorf("request creation error: %s", err)
	}
	msg.WriteTo(req.Header)

	done := make(chan struct{})
	go func() {
		cw := &countWriter{pw, 0}
		err := r.Write(cw)
		if err != nil {
			s.log.Debug("Write to pipe failed: %s", err)
		}
		TransferLog.Debug("Coppied %d bytes from %s", cw.count, "local to remote")
		close(done)
	}()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxy request error: %s", err)
	}

	inner, err := http.ReadResponse(bufio.NewReader(resp.Body), r)
	if err != nil {
		return fmt.Errorf("reading response error: %s", msg, host, err)
	}
	copyHeader(w.Header(), inner.Header)
	w.WriteHeader(inner.StatusCode)
	if inner.Body != nil {
		transfer("remote to local", w, inner.Body)
	}

	<-done
	s.log.Debug("Done %s", msg)
	return nil
}

func (s *Server) proxyConn(host string, c net.Conn, msg *proto.ControlMessage) error {
	s.log.Debug("Start %s", msg)
	defer c.Close()

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := http.NewRequest(http.MethodPut, url(host), pr)
	if err != nil {
		return fmt.Errorf("request creation error: %s", err)
	}
	msg.WriteTo(req.Header)

	done := make(chan struct{})
	go func() {
		transfer("local to remote", pw, c)
		close(done)
	}()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxy request error: %s", err)
	}

	transfer("remote to local", c, resp.Body)

	<-done
	s.log.Debug("Done %s", msg)
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}
