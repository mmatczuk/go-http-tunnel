// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// ServerConfig defines configuration for the Server.
type ServerConfig struct {
	// RemoteAddr is TCP address to listen for controller connections. If empty ":0"
	// is used.
	Addr string
	// AutoSubscribe if enabled will automatically subscribe new clients on
	// first call.
	AutoSubscribe bool
	// TLSConfig specifies the tls configuration to use with tls.Listener.
	TLSConfig *tls.Config
	// Listener specifies optional listener for controller connections. If nil
	// tls.Listen("tcp", RemoteAddr, TLSConfig) is used.
	Listener net.Listener
	// Logger is optional logger. If nil logging is disabled.
	Logger log.Logger
	// RegisteredClient get
	RegisteredClientsProvider RegisteredClientsProvider
}

// Server is responsible for proxying public connections to the controller over a
// tunnel connection.
type Server struct {
	*registry
	config *ServerConfig

	listener   net.Listener
	connPool   *connPool
	httpClient *http.Client
	logger     log.Logger
}

// NewServer creates a new Server.
func NewServer(config *ServerConfig) (*Server, error) {
	listener, err := listener(config)
	if err != nil {
		return nil, fmt.Errorf("listener failed: %s", err)
	}

	logger := config.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}

	s := &Server{
		registry: newRegistry(logger),
		config:   config,
		listener: listener,
		logger:   logger,
	}

	t := &http2.Transport{}
	pool := newConnPool(t, s.disconnected)
	t.ConnPool = pool
	s.connPool = pool
	s.httpClient = &http.Client{
		Transport: t,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return s, nil
}

func listener(config *ServerConfig) (net.Listener, error) {
	if config.Listener != nil {
		return config.Listener, nil
	}

	if config.Addr == "" {
		return nil, errors.New("missing RemoteAddr")
	}
	if config.TLSConfig == nil {
		return nil, errors.New("missing TLSConfig")
	}

	return net.Listen("tcp", config.Addr)
}

// disconnected clears resources used by controller, it's invoked by connection pool
// when controller goes away.
func (s *Server) disconnected(identifier id.ID) {
	s.logger.Log(
		"level", 1,
		"action", "disconnected",
		"identifier", identifier,
	)

	i := s.registry.clear(identifier)
	if i == nil {
		return
	}
	for _, l := range i.Listeners {
		s.logger.Log(
			"level", 2,
			"action", "close listener",
			"identifier", identifier,
			"addr", l.Addr(),
		)
		l.Close()
	}
}

// Start starts accepting connections form clients. For accepting http traffic
// from end users server must be run as handler on http server.
func (s *Server) Start() {
	addr := s.listener.Addr().String()

	s.logger.Log(
		"level", 1,
		"action", "start",
		"addr", addr,
	)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				s.logger.Log(
					"level", 1,
					"action", "control connection listener closed",
					"addr", addr,
				)
				return
			}

			s.logger.Log(
				"level", 0,
				"msg", "accept of control connection failed",
				"addr", addr,
				"err", err,
			)
			continue
		}

		if err := keepAlive(conn); err != nil {
			s.logger.Log(
				"level", 0,
				"msg", "TCP keepalive for control connection failed",
				"addr", addr,
				"err", err,
			)
		}

		go s.handleClient(tls.Server(conn, s.config.TLSConfig), true)
	}
}

func (s *Server) handleClient(conn net.Conn, main bool) {
	logger := log.NewContext(s.logger).With("addr", conn.RemoteAddr())

	logger.Log(
		"level", 1,
		"action", "try connect",
	)

	var (
		err        error
		inConnPool bool
		ok         bool
		ID         id.ID
		cfg        *RegisteredClientConfig
		clientInfo *RegisteredClientInfo
		controller *clientConnectionController
		body       io.Reader
		req        *http.Request
		resp       *http.Response
		tlsConn    *tls.Conn
		ccon       *clientConnection
	)

	if tlsConn, ok = conn.(*tls.Conn); !ok {
		logger.Log(
			"level", 0,
			"msg", "invalid connection type",
			"err", fmt.Errorf("expected TLS controller, got %T", conn),
		)
		goto Reject
	}

	ID, err = id.PeerID(tlsConn)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "certificate error",
			"err", err,
		)
		goto Reject
	}

	logger = logger.With("identifier", ID)

	if s.config.AutoSubscribe {
		s.Subscribe(ID)
	} else if s.config.RegisteredClientsProvider != nil {
		if cfg, err = s.config.RegisteredClientsProvider.Get(ID); err == nil {
			if cfg.Disabled {
				logger.Log(
					"level", 2,
					"msg", "Client has be disabled",
				)
				goto Reject
			}
			if s.connPool.Has(ID) {
				if ccon, err = s.connPool.AddClientConnection(ID, conn); err != nil {
					goto Reject
				}
				logger.Log(
					"level", 1,
					"msg", fmt.Sprintf("new connection #%d for client added", ccon.id),
				)
				return
			} else {
				s.Subscribe(ID)
			}
		} else if IsNotRegistered(err) {
			logger.Log(
				"level", 2,
				"msg", err.Error(),
			)
			goto Reject
		} else {
			logger.Log(
				"level", 2,
				"msg", "Get registered controller failed",
				"err", err,
			)
			goto Reject
		}
	} else if !s.IsSubscribed(ID) {
		logger.Log(
			"level", 2,
			"msg", "unknown controller",
		)
		goto Reject
	}

	if err = conn.SetDeadline(time.Time{}); err != nil {
		logger.Log(
			"level", 2,
			"msg", "setting infinite deadline failed",
			"err", err,
		)
		goto Reject
	}

	controller = &clientConnectionController{cfg, conn, ID, logger}

	if err := s.connPool.AddConn(controller); err != nil {
		logger.Log(
			"level", 2,
			"msg", "adding connection failed",
			"err", err,
		)
		goto Reject
	}

	inConnPool = true

	req, err = http.NewRequest(http.MethodConnect, s.connPool.URL(controller.ID), body)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake request creation failed",
			"err", err,
		)
		goto Reject
	}

	if cfg != nil && cfg.Connections > 0 {
		req.Header.Set(HeaderConnectionsCount, strconv.Itoa(int(controller.cfg.Connections)))
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err = s.httpClient.Do(req)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto Reject
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Status %s", resp.Status)
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto Reject
	}

	if resp.ContentLength == 0 {
		err = fmt.Errorf("Tunnels Content-Legth: 0")
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto Reject
	}

	if cfg == nil {
		if err = json.NewDecoder(&io.LimitedReader{R: resp.Body, N: 126976}).Decode(cfg); err != nil {
			logger.Log(
				"level", 2,
				"msg", "handshake failed",
				"err", err,
			)
			goto Reject
		}
		// Ony main connection
		cfg.Connections = 0
	} else {
		clientInfo = &RegisteredClientInfo{}
		if err = json.NewDecoder(&io.LimitedReader{R: resp.Body, N: 126976}).Decode(clientInfo); err != nil {
			logger.Log(
				"level", 2,
				"msg", "handshake failed",
				"err", err,
			)
			goto Reject
		}

		cfg.Hostname = clientInfo.Hostname
		for name, ct := range clientInfo.Tunnels {
			if t, ok := cfg.Tunnels[name]; ok {
				if ct.Disabled {
					t.Disabled = true
				} else {
					t.LocalAddr = ct.LocalAddr
					t.Auth = ct.Auth
				}
			} else {
				logger.Log(
					"level", 1,
					"msg", fmt.Sprintf("tunnel %q not registered", name),
				)
				delete(clientInfo.Tunnels, name)
			}
		}
	}

	if len(cfg.Tunnels) == 0 {
		err = fmt.Errorf("No tunnels")
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto Reject
	}

	if err = s.addTunnels(cfg.Tunnels, ID); err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto Reject
	}

	logger.Log(
		"level", 1,
		"action", "connected",
	)

	return
Reject:
	logger.Log(
		"level", 1,
		"action", "rejected",
	)

	if inConnPool {
		s.notifyError(err, ID)
		s.connPool.DeleteConn(ID)
	}

	conn.Close()
}

// notifyError tries to send error to controller.
func (s *Server) notifyError(serverError error, identifier id.ID) {
	if serverError == nil {
		return
	}

	req, err := http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), nil)
	if err != nil {
		s.logger.Log(
			"level", 2,
			"action", "controller error notification failed",
			"identifier", identifier,
			"err", err,
		)
		return
	}

	req.Header.Set(proto.HeaderError, serverError.Error())

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	s.httpClient.Do(req.WithContext(ctx))
}

// addTunnels invokes addHost or addListener based on data from proto.Tunnel. If
// a tunnel cannot be added whole batch is reverted.
func (s *Server) addTunnels(tunnels map[string]*proto.Tunnel, identifier id.ID) error {
	i := &RegistryItem{
		Hosts:     []*HostAuth{},
		Listeners: []*Listener{},
	}

	var err error
	for name, t := range tunnels {
		if t.Disabled {
			continue
		}

		switch t.Protocol {
		case proto.HTTP:
			i.Hosts = append(i.Hosts, &HostAuth{t, NewAuth(t.Auth)})
		case proto.TCP, proto.TCP4, proto.TCP6, proto.UNIX:
			var l net.Listener
			l, err = net.Listen(t.Protocol, t.RemoteAddr)
			if err != nil {
				goto rollback
			}

			s.logger.Log(
				"level", 2,
				"action", "open listener",
				"identifier", identifier,
				"addr", l.Addr(),
			)

			i.Listeners = append(i.Listeners, &Listener{l, t})
		default:
			err = fmt.Errorf("unsupported protocol for tunnel %s: %s", name, t.Protocol)
			goto rollback
		}
	}

	err = s.set(i, identifier)
	if err != nil {
		goto rollback
	}

	for _, l := range i.Listeners {
		go s.listen(l, identifier)
	}

	return nil

rollback:
	for _, l := range i.Listeners {
		l.Close()
	}

	return err
}

// Unsubscribe removes controller from registry, disconnects controller if already
// connected and returns it's RegistryItem.
func (s *Server) Unsubscribe(identifier id.ID) *RegistryItem {
	s.connPool.DeleteConn(identifier)
	return s.registry.Unsubscribe(identifier)
}

// Ping measures the RTT response time.
func (s *Server) Ping(identifier id.ID) (time.Duration, error) {
	return s.connPool.Ping(identifier)
}

func (s *Server) listen(l *Listener, identifier id.ID) {
	addr := l.Addr().String()

	for {
		conn, err := l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				s.logger.Log(
					"level", 2,
					"action", "listener closed",
					"identifier", identifier,
					"addr", addr,
				)
				return
			}

			s.logger.Log(
				"level", 0,
				"msg", "accept of connection failed",
				"identifier", identifier,
				"addr", addr,
				"err", err,
			)
			continue
		}

		msg := &proto.ControlMessage{
			LocalAddr:      l.Tunnel.LocalAddr,
			Action:         proto.ActionProxy,
			ForwardedHost:  l.Addr().String(),
			ForwardedProto: l.Addr().Network(),
		}

		if err := keepAlive(conn); err != nil {
			s.logger.Log(
				"level", 1,
				"msg", "TCP keepalive for tunneled connection failed",
				"identifier", identifier,
				"ctrlMsg", msg,
				"err", err,
			)
		}

		go func() {
			if err := s.proxyConn(identifier, conn, msg); err != nil {
				s.logger.Log(
					"level", 0,
					"msg", "proxy error",
					"identifier", identifier,
					"ctrlMsg", msg,
					"err", err,
				)
			}
		}()
	}
}

// ServeHTTP proxies http connection to the controller.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := s.RoundTrip(r)
	if err == errUnauthorised {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"User Visible Realm\"")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err != nil {
		s.logger.Log(
			"level", 0,
			"action", "round trip failed",
			"addr", r.RemoteAddr,
			"host", r.Host,
			"url", r.URL,
			"err", err,
		)

		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	transfer(w, resp.Body, log.NewContext(s.logger).With(
		"dir", "controller to user",
		"dst", r.RemoteAddr,
		"src", r.Host,
	))
}

// RoundTrip is http.RoundTriper implementation.
func (s *Server) RoundTrip(r *http.Request) (*http.Response, error) {
	identifier, hostInfo, ok := s.Subscriber(r.Host)
	if !ok {
		return nil, errClientNotSubscribed
	}

	outr := r.WithContext(r.Context())
	if r.ContentLength == 0 {
		outr.Body = nil // Issue 16036: nil Body for http.Transport retries
	}
	outr.Header = cloneHeader(r.Header)

	if hostInfo.auth != nil {
		user, password, _ := r.BasicAuth()
		if hostInfo.auth.User != user || hostInfo.auth.Password != password {
			return nil, errUnauthorised
		}
		outr.Header.Del("Authorization")
	}

	setXForwardedFor(outr.Header, r.RemoteAddr)

	scheme := r.URL.Scheme
	if scheme == "" {
		if r.TLS != nil {
			scheme = proto.HTTPS
		} else {
			scheme = proto.HTTP
		}
	}
	if r.Header.Get("X-Forwarded-Host") == "" {
		outr.Header.Set("X-Forwarded-Host", r.Host)
		outr.Header.Set("X-Forwarded-Proto", scheme)
	}

	msg := &proto.ControlMessage{
		LocalAddr:      hostInfo.tunnel.LocalAddr,
		Action:         proto.ActionProxy,
		ForwardedHost:  r.Host,
		ForwardedProto: scheme,
	}

	return s.proxyHTTP(identifier, outr, msg)
}

func (s *Server) proxyConn(identifier id.ID, conn net.Conn, msg *proto.ControlMessage) error {
	s.logger.Log(
		"level", 2,
		"action", "proxy controller",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	defer conn.Close()

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := s.connectRequest(identifier, msg, pr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		transfer(pw, conn, log.NewContext(s.logger).With(
			"dir", "user to controller",
			"dst", identifier,
			"src", conn.RemoteAddr(),
		))
		cancel()
		close(done)
	}()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("io error: %s", err)
	}
	defer resp.Body.Close()

	transfer(conn, resp.Body, log.NewContext(s.logger).With(
		"dir", "controller to user",
		"dst", conn.RemoteAddr(),
		"src", identifier,
	))

	<-done

	s.logger.Log(
		"level", 2,
		"action", "proxy controller done",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	return nil
}

func (s *Server) proxyHTTP(identifier id.ID, r *http.Request, msg *proto.ControlMessage) (*http.Response, error) {
	s.logger.Log(
		"level", 2,
		"action", "proxy HTTP",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := s.connectRequest(identifier, msg, pr)
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
				"identifier", identifier,
				"ctrlMsg", msg,
				"err", err,
			)
		}

		s.logger.Log(
			"level", 3,
			"action", "transferred",
			"identifier", identifier,
			"bytes", cw.count,
			"dir", "user to controller",
			"dst", r.Host,
			"src", r.RemoteAddr,
		)

		if r.Body != nil {
			r.Body.Close()
		}
	}()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("io error: %s", err)
	}

	s.logger.Log(
		"level", 2,
		"action", "proxy HTTP done",
		"identifier", identifier,
		"ctrlMsg", msg,
		"status code", resp.StatusCode,
	)

	return resp, nil
}

// connectRequest creates HTTP request to controller with a given identifier having
// control message and data input stream, output data stream results from
// response the created request.
func (s *Server) connectRequest(identifier id.ID, msg *proto.ControlMessage, r io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPut, s.connPool.URL(identifier), r)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %s", err)
	}
	msg.WriteToHeader(req.Header)

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
