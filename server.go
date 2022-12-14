// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/hons82/go-http-tunnel/connection"
	"github.com/hons82/go-http-tunnel/fileutil"
	"github.com/hons82/go-http-tunnel/id"
	"github.com/hons82/go-http-tunnel/log"
	"github.com/hons82/go-http-tunnel/proto"
	"github.com/inconshreveable/go-vhost"
)

// SubscriptionListener A set of listeners to manage subscribers
type SubscriptionListener interface {
	// Invoked if AutoSubscribe is false and must return true if the client is allowed to subscribe or not.
	// If the tlsConfig is configured to require client certificate validation, chain will contain the first
	// verified chain, else the presented peer certificate.
	CanSubscribe(id id.ID, chain []*x509.Certificate) bool
	// Invoked when the client has been subscribed.
	// If the tlsConfig is configured to require client certificate validation, chain will contain the first
	// verified chain, else the presented peer certificate.
	Subscribed(id id.ID, tlsConn *tls.Conn, chain []*x509.Certificate)
	// Invoked before the client is unsubscribed.
	Unsubscribed(id id.ID)
}

// ServerConfig defines configuration for the Server.
type ServerConfig struct {
	// Addr is TCP address to listen for client connections. If empty ":0" is used.
	Addr string
	// AutoSubscribe if enabled will automatically subscribe new clients on first call.
	AutoSubscribe bool
	// TLSConfig specifies the tls configuration to use with tls.Listener.
	TLSConfig *tls.Config
	// Listener specifies optional listener for client connections. If nil tls.Listen("tcp", Addr, TLSConfig) is used.
	Listener net.Listener
	// Logger is optional logger. If nil logging is disabled.
	Logger log.Logger
	// Addr is TCP address to listen for TLS SNI connections
	SNIAddr string
	// Used to configure the keepalive for the server -> client tcp connection
	KeepAlive connection.KeepAliveConfig
	// How long should a disconnected message been hold before sending it to the log
	Debounce Debounced
	// Optional listener to manage subscribers
	SubscriptionListener SubscriptionListener
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
	debounce   Debounced
	vhostMuxer *vhost.TLSMuxer
}

// Debounced Hold IDs that are disconnected for a short time before executing the function.
type Debounced struct {
	Execute         func(f func())
	disconnectedIDs []id.ID
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
		debounce: config.Debounce,
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

	if config.SNIAddr != "" {
		l, err := net.Listen("tcp", config.SNIAddr)
		if err != nil {
			return nil, err
		}
		mux, err := vhost.NewTLSMuxer(l, DefaultTimeout)
		if err != nil {
			return nil, fmt.Errorf("SNI Muxer creation failed: %s", err)
		}
		s.vhostMuxer = mux
		go func() {
			for {
				conn, err := mux.NextError()
				vhostName := ""
				tlsConn, ok := conn.(*vhost.TLSConn)
				if ok {
					vhostName = tlsConn.Host()
				}

				switch err.(type) {
				case vhost.BadRequest:
					logger.Log(
						"level", 0,
						"action", "got a bad request!",
						"addr", conn.RemoteAddr(),
					)
				case vhost.NotFound:

					logger.Log(
						"level", 0,
						"action", "got a connection for an unknown vhost",
						"addr", vhostName,
					)
				case vhost.Closed:
					logger.Log(
						"level", 0,
						"action", "closed conn",
						"addr", vhostName,
					)
				}

				if conn != nil {
					conn.Close()
				}
			}
		}()
	}

	return s, nil
}

func listener(config *ServerConfig) (net.Listener, error) {
	if config.Listener != nil {
		return config.Listener, nil
	}

	if config.Addr == "" {
		return nil, errors.New("missing Addr")
	}
	if config.TLSConfig == nil {
		return nil, errors.New("missing TLSConfig")
	}

	return net.Listen("tcp", config.Addr)
}

// disconnected clears resources used by client, it's invoked by connection pool when client goes away.
func (s *Server) disconnected(identifier id.ID) {
	if s.debounce.Execute != nil {
		s.debounce.disconnectedIDs = append(s.debounce.disconnectedIDs, identifier)

		s.debounce.Execute(func() {
			for _, id := range s.debounce.disconnectedIDs {
				s.logger.Log(
					"level", 1,
					"action", "disconnected",
					"identifier", id,
				)
			}
			s.debounce.disconnectedIDs = nil
		})
	} else {
		s.logger.Log(
			"level", 1,
			"action", "disconnected",
			"identifier", identifier,
		)
	}

	i := s.registry.Unsubscribe(identifier, s.config.AutoSubscribe)
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

		s.logger.Log(
			"level", 2,
			"msg", fmt.Sprintf("setting up keep alive using config: %v", s.config.KeepAlive.String()),
		)

		if err := s.config.KeepAlive.Set(conn); err != nil {
			s.logger.Log(
				"level", 0,
				"msg", "TCP keepalive for control connection failed",
				"addr", addr,
				"err", err,
			)
		}

		go s.handleClient(tls.Server(conn, s.config.TLSConfig))
	}
}

func (s *Server) handleClient(conn net.Conn) {
	logger := log.NewContext(s.logger).With("remote addr", conn.RemoteAddr())

	logger.Log(
		"level", 2,
		"action", "try connect",
	)

	var (
		identifier id.ID
		IDInfo     id.IDInfo
		req        *http.Request
		resp       *http.Response
		tunnels    map[string]*proto.Tunnel
		err        error
		ok         bool

		inConnPool bool
		certs      []*x509.Certificate

		remainingIDs []id.ID
		found        bool
	)

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		logger.Log(
			"level", 0,
			"msg", "invalid connection type",
			"err", fmt.Errorf("expected TLS conn, got %T", conn),
		)
		goto reject
	}

	identifier, IDInfo, err = id.PeerID(tlsConn)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "certificate error",
			"err", err,
		)
		goto reject
	}

	logger = logger.With("identifier", identifier)

	certs = tlsConn.ConnectionState().PeerCertificates
	if tlsConn.ConnectionState().VerifiedChains != nil && len(tlsConn.ConnectionState().VerifiedChains) > 0 {
		certs = tlsConn.ConnectionState().VerifiedChains[0]
	}
	if s.config.AutoSubscribe {
		s.Subscribe(identifier)
		if s.config.SubscriptionListener != nil {
			s.config.SubscriptionListener.Subscribed(identifier, tlsConn, certs)
		}
	} else if !s.IsSubscribed(identifier) {
		if s.config.SubscriptionListener != nil && s.config.SubscriptionListener.CanSubscribe(identifier, certs) {
			s.Subscribe(identifier)
			s.config.SubscriptionListener.Subscribed(identifier, tlsConn, certs)
		} else {
			logger.Log(
				"level", 2,
				"msg", "unknown client",
			)
			goto reject
		}
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
	inConnPool = true

	req, err = http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), nil)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake request creation failed",
			"err", err,
		)
		goto reject
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
		goto reject
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status %s", resp.Status)
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if resp.ContentLength == 0 {
		err = fmt.Errorf("tunnels content-length: 0")
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if err = json.NewDecoder(&io.LimitedReader{R: resp.Body, N: 126976}).Decode(&tunnels); err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if len(tunnels) == 0 {
		logger.Log(
			"level", 1,
			"msg", "configuration error",
			"err", fmt.Errorf("no tunnels"),
		)
		goto reject
	}

	if err = s.hasTunnels(tunnels, identifier); err != nil {
		logger.Log(
			"level", 2,
			"msg", "tunnel check failed",
			"err", err,
		)
		goto reject
	}

	if err = s.addTunnels(tunnels, identifier, IDInfo); err != nil {
		logger.Log(
			"level", 2,
			"msg", "add tunnel failed",
			"err", err,
		)
		goto reject
	}

	remainingIDs, found = id.Remove(s.debounce.disconnectedIDs, identifier)
	if found {
		s.debounce.disconnectedIDs = remainingIDs
		logger.Log(
			"level", 2,
			"action", "reconnected",
		)
	} else {
		logger.Log(
			"level", 1,
			"action", "connected",
		)
	}

	return

reject:
	logger.Log(
		"level", 1,
		"action", "rejected",
	)

	if inConnPool {
		s.notifyError(err, identifier)
		s.connPool.DeleteConn(identifier)
	}

	conn.Close()
}

// loadAllowedTunnels registers allowed tunnels from a file
func (s *Server) loadAllowedTunnels(propertiesFile string) {
	clients, err := fileutil.ReadPropertiesFile(propertiesFile)
	if err != nil {
		s.logger.Log(
			"level", 1,
			"action", "failed to load clients",
			"err", err,
		)
		return
	}

	for host, value := range clients {
		if err := s.registerTunnel(host, value); err != nil {
			s.logger.Log(
				"level", 2,
				"action", "failed to load tunnel",
				"host", host,
				"err", err,
			)
		}
	}
}

// ReloadTunnels registers allowed tunnels from a file
func (s *Server) ReloadTunnels(path string) {
	directory, err := fileutil.IsDirectory(path)
	if err != nil {
		s.logger.Log(
			"level", 3,
			"action", "could not determine if path is a directory",
			"err", err,
		)
	}
	if directory {
		files, err := os.ReadDir(path)
		if err != nil {
			s.logger.Log(
				"level", 2,
				"action", "could not read directory",
				"err", err,
			)
		}
		for _, file := range files {
			if file.IsDir() {
				s.logger.Log(
					"level", 3,
					"action", "skip directory",
					"file", file.Name(),
				)
			} else {
				s.loadAllowedTunnels(filepath.Join(path, file.Name()))
			}
		}
	} else {
		s.loadAllowedTunnels(path)
	}
}

// notifyError tries to send error to client.
func (s *Server) notifyError(serverError error, identifier id.ID) {
	if serverError == nil {
		return
	}

	req, err := http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), nil)
	if err != nil {
		s.logger.Log(
			"level", 2,
			"action", "client error notification failed",
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

func (s *Server) hasTunnels(tunnels map[string]*proto.Tunnel, identifier id.ID) error {
	var err error
	for name, t := range tunnels {
		// Check the current tunnel
		// AutoSubscribe --> Tunnel not yet registered (means that it isn't already opened)
		// !AutoSubscribe -> Tunnel has to be already registered, and therefore allowed to be opened
		if s.config.AutoSubscribe == s.HasTunnel(t.Host, identifier) {
			err = fmt.Errorf("tunnel %s (%s) not allowed for %s", name, t.Host, identifier)
			break
		}
	}
	return err
}

// addTunnels invokes addHost or addListener based on data from proto.Tunnel. If
// a tunnel cannot be added whole batch is reverted.
func (s *Server) addTunnels(tunnels map[string]*proto.Tunnel, identifier id.ID, IDInfo id.IDInfo) error {
	i := &RegistryItem{
		IDInfo:    &IDInfo,
		Hosts:     []*HostAuth{},
		Listeners: []net.Listener{},
	}

	var err error
	for name, t := range tunnels {
		switch t.Protocol {
		case proto.HTTP:
			i.Hosts = append(i.Hosts, &HostAuth{t.Host, NewAuth(t.Auth)})
		case proto.TCP, proto.TCP4, proto.TCP6, proto.UNIX:
			var l net.Listener
			l, err = net.Listen(t.Protocol, t.Addr)
			if err != nil {
				goto rollback
			}

			s.logger.Log(
				"level", 2,
				"action", "open listener",
				"identifier", identifier,
				"addr", l.Addr(),
			)

			i.Listeners = append(i.Listeners, l)
		case proto.SNI:
			if s.vhostMuxer == nil {
				err = fmt.Errorf("unable to configure SNI for tunnel %s: %s", name, t.Protocol)
				goto rollback
			}
			var l net.Listener
			l, err = s.vhostMuxer.Listen(t.Host)
			if err != nil {
				goto rollback
			}

			s.logger.Log(
				"level", 2,
				"action", "add SNI vhost",
				"identifier", identifier,
				"host", t.Host,
			)

			i.Listeners = append(i.Listeners, l)
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

// Unsubscribe removes client from registry, disconnects client if already
// connected and returns it's RegistryItem.
func (s *Server) Unsubscribe(identifier id.ID) *RegistryItem {
	if s.config.SubscriptionListener != nil {
		s.config.SubscriptionListener.Unsubscribed(identifier)
	}
	s.connPool.DeleteConn(identifier)
	return s.registry.Unsubscribe(identifier, s.config.AutoSubscribe)
}

// Ping measures the RTT response time.
func (s *Server) Ping(identifier id.ID) (time.Duration, error) {
	return s.connPool.Ping(identifier)
}

func (s *Server) listen(l net.Listener, identifier id.ID) {
	addr := l.Addr().String()

	for {
		conn, err := l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "Listener closed") {
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
			Action:         proto.ActionProxy,
			ForwardedProto: l.Addr().Network(),
		}

		tlsConn, ok := conn.(*vhost.TLSConn)

		s.logger.Log(
			"level", 1,
			"msg", fmt.Sprintf("setting up keep alive using config: %v", s.config.KeepAlive.String()),
		)

		if ok {
			msg.ForwardedHost = tlsConn.Host()
			err = s.config.KeepAlive.Set(tlsConn.Conn)

		} else {
			msg.ForwardedHost = l.Addr().String()
			err = s.config.KeepAlive.Set(conn)
		}

		if err != nil {
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

// Upgrade the connection
func (s *Server) Upgrade(identifier id.ID, conn net.Conn, requestBytes []byte) error {

	var err error

	msg := &proto.ControlMessage{
		Action:         proto.ActionProxy,
		ForwardedProto: "https",
	}

	tlsConn, ok := conn.(*tls.Conn)
	if ok {
		msg.ForwardedHost = tlsConn.ConnectionState().ServerName
		err = s.config.KeepAlive.Set(tlsConn.NetConn())

	} else {
		msg.ForwardedHost = conn.RemoteAddr().String()
		err = s.config.KeepAlive.Set(conn)
	}

	if err != nil {
		s.logger.Log(
			"level", 1,
			"msg", "TCP keepalive for tunneled connection failed",
			"identifier", identifier,
			"ctrlMsg", msg,
			"err", err,
		)
	}

	go func() {
		if err := s.proxyConnUpgraded(identifier, conn, msg, requestBytes); err != nil {
			s.logger.Log(
				"level", 0,
				"msg", "proxy error",
				"identifier", identifier,
				"ctrlMsg", msg,
				"err", err,
			)
		}
	}()

	return nil
}

// ServeHTTP proxies http connection to the client.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Method, "TRACE") {
		s.logger.Log(
			"level", 2,
			"action", "method not allowed",
			"method", r.Method,
			"addr", r.RemoteAddr,
			"host", r.Host,
			"url", r.URL,
		)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp, err := s.RoundTrip(r)
	if err != nil {
		level := 0
		code := http.StatusBadGateway
		if err == errUnauthorised {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"User Visible Realm\"")
			level = 1
			code = http.StatusUnauthorized
		} else if err == errClientNotSubscribed {
			level = 2
			code = http.StatusNotFound
		}
		s.logger.Log(
			"level", level,
			"action", "round trip failed",
			"addr", r.RemoteAddr,
			"host", r.Host,
			"url", r.URL,
			"code", code,
		)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(code)
		fmt.Fprintln(w, err.Error())
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	transfer(w, resp.Body, log.NewContext(s.logger).With(
		"dir", "client to user",
		"dst", r.RemoteAddr,
		"src", r.Host,
	))
}

// RoundTrip is http.RoundTriper implementation.
func (s *Server) RoundTrip(r *http.Request) (*http.Response, error) {
	identifier, auth, ok := s.Subscriber(r.Host)
	if !ok {
		return nil, errClientNotSubscribed
	}

	outr := r.WithContext(r.Context())
	if r.ContentLength == 0 {
		outr.Body = nil // Issue 16036: nil Body for http.Transport retries
	}
	outr.Header = cloneHeader(r.Header)

	if auth != nil {
		user, password, _ := r.BasicAuth()
		if auth.User != user || auth.Password != password {
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
		Action:         proto.ActionProxy,
		ForwardedHost:  r.Host,
		ForwardedProto: scheme,
	}

	return s.proxyHTTP(identifier, outr, msg)
}

func (s *Server) proxyConnUpgraded(identifier id.ID, conn net.Conn, msg *proto.ControlMessage, requestBytes []byte) error {
	s.logger.Log(
		"level", 2,
		"action", "proxy conn",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	defer conn.Close()

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	continueChan := make(chan int)

	go func() {
		pw.Write(requestBytes)
		continueChan <- 1
	}()

	req, err := s.connectRequest(identifier, msg, pr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		<-continueChan
		transfer(pw, conn, log.NewContext(s.logger).With(
			"dir", "user to client",
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
		"dir", "client to user",
		"dst", conn.RemoteAddr(),
		"src", identifier,
	))

	select {
	case <-done:
	case <-time.After(DefaultTimeout):
	}

	s.logger.Log(
		"level", 2,
		"action", "proxy conn done",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	return nil
}

func (s *Server) proxyConn(identifier id.ID, conn net.Conn, msg *proto.ControlMessage) error {
	s.logger.Log(
		"level", 2,
		"action", "proxy conn",
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
			"dir", "user to client",
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
		"dir", "client to user",
		"dst", conn.RemoteAddr(),
		"src", identifier,
	))

	select {
	case <-done:
	case <-time.After(DefaultTimeout):
	}

	s.logger.Log(
		"level", 2,
		"action", "proxy conn done",
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

// connectRequest creates HTTP request to client with a given identifier having
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

// ListenerInfo info about the listener
type ListenerInfo struct {
	Network string
	Addr    string
}

// ClientInfo info about the client
type ClientInfo struct {
	ID        string
	IDInfo    id.IDInfo
	Listeners []*ListenerInfo
	Hosts     []string
}

// GetClientInfo prepare and get client info
func (s *Server) GetClientInfo() []*ClientInfo {
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()
	ret := []*ClientInfo{}
	for k, v := range s.registry.items {
		c := &ClientInfo{
			ID: k.String(),
		}
		ret = append(ret, c)
		if v == voidRegistryItem {
			s.logger.Log(
				"level", 3,
				"identifier", k.String(),
				"msg", "void registry item",
			)
		} else {
			c.IDInfo = *v.IDInfo
			for _, l := range v.Hosts {
				c.Hosts = append(c.Hosts, l.Host)
			}
			for _, l := range v.Listeners {
				p := &ListenerInfo{Network: l.Addr().Network(), Addr: l.Addr().String()}
				c.Listeners = append(c.Listeners, p)
			}
		}
	}
	return ret
}
