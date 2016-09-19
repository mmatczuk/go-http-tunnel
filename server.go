package h2tun

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/andrew-d/id"
	"github.com/koding/logging"
	"golang.org/x/net/http2"
)

// TODO mma add ListenerFunc func(net.Listener) net.Listener to allow for tls listener decoration
// TODO mma add dynamic allowed clients modifications
type Server struct {
	allowedClients []*AllowedClient
	listener       net.Listener

	httpClient *http.Client
	hostConn   map[string]net.Conn
	hostConnMu sync.RWMutex

	tcpPorts map[int]*AllowedClient

	log logging.Logger
}

type AllowedClient struct {
	ID        id.ID
	Host      string
	Listeners []net.Listener
}

func NewServer(tlsConfig *tls.Config, allowedClients []*AllowedClient) (*Server, error) {
	l, err := tls.Listen("tcp", ":0", tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("tls listener failed :%s", err)
	}

	s := &Server{
		allowedClients: allowedClients,
		listener:       l,
		log:            logging.NewLogger("server"),
	}
	s.initHTTPClient()

	return s, nil
}

func (s *Server) initHTTPClient() {
	// TODO mma try using connection pool for transport
	s.hostConn = make(map[string]net.Conn)
	s.httpClient = &http.Client{
		Transport: &http2.Transport{
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				s.hostConnMu.RLock()
				defer s.hostConnMu.RUnlock()

				conn, ok := s.hostConn[addr]
				if !ok {
					return nil, fmt.Errorf("no connection for %q", addr)
				}
				return conn, nil
			},
		},
	}
}

func (s *Server) Start() {
	go s.listenControl()
	s.listenClientListeners()
}

func (s *Server) listenControl() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.log.Warning("Accept failed: %s", err)
			continue
		}
		s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	s.log.Info("New client %s", conn.RemoteAddr().String())

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
		goto cleanup
	}

	client, ok = s.checkID(id)
	if !ok {
		s.log.Warning("Unknown certificate: %q", id.String())
		goto cleanup
	}

	req, err = http.NewRequest(http.MethodConnect, url(client, ""), nil)
	if err != nil {
		s.log.Error("Invalid host %q for client %q", client.Host, client.ID)
		goto cleanup
	}

	if err = conn.SetDeadline(time.Time{}); err != nil {
		s.log.Warning("Setting no deadline failed: %s", err)
		// recoverable
	}

	if err := s.addHostConn(client, conn); err != nil {
		s.log.Warning("Could not add host: %s", err)
		goto cleanup
	}

	resp, err = s.httpClient.Do(req)
	if err != nil {
		s.log.Warning("Handshake failed %s", err)
		goto cleanup
	}
	if resp.StatusCode != http.StatusOK {
		s.log.Warning("Handshake failed")
		goto cleanup
	}

	return

cleanup:
	conn.Close()
	if client != nil {
		s.deleteHostConn(client.Host)
	}
}

func (s *Server) addHostConn(client *AllowedClient, conn net.Conn) error {
	key := hostPort(client.Host)

	s.hostConnMu.Lock()
	defer s.hostConnMu.Unlock()

	if c, ok := s.hostConn[key]; ok {
		return fmt.Errorf("client %q already connected from %q", client.ID, c.RemoteAddr().String())
	}

	s.hostConn[key] = conn

	return nil
}

func (s *Server) deleteHostConn(host string) {
	s.hostConnMu.Lock()
	delete(s.hostConn, hostPort(host))
	s.hostConnMu.Unlock()
}

func hostPort(host string) string {
	// TODO mma add support for custom ports
	return fmt.Sprint(host, ":443")
}

func (s *Server) checkID(id id.ID) (*AllowedClient, bool) {
	for _, c := range s.allowedClients {
		if id.Equals(c.ID) {
			return c, true
		}
	}
	return nil, false
}

func (s *Server) listenClientListeners() {
	for _, client := range s.allowedClients {
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
			s.log.Warning("Accept failed: %s", err)
			continue
		}
		s.log.Debug("Accepted connection from %q", conn.RemoteAddr().String())

		// TODO mma get Protocol from Network
		// TODO mma get LocalIP from Addr
		msg := &ControlMessage{
			Action:   RequestClientSession,
			Protocol: RAW,
		}

		go s.proxy(conn, client, msg)
	}
}

func (s *Server) proxy(conn net.Conn, client *AllowedClient, msg *ControlMessage) {
	defer conn.Close()

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := http.NewRequest(http.MethodPut, url(client, ""), pr)
	if err != nil {
		s.log.Error("Request creation failed: %s", err)
		return
	}

	// read from caller, write to tunnel client
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		transfer("local to remote", pw, conn, s.log)
		wg.Done()
	}()

	// read from tunnel client, write to caller
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.log.Error("Proxing conn from %q to %q failed: %s", conn.RemoteAddr().String(), client.Host, err)
		return
	}
	transfer("remote to local", conn, resp.Body, s.log)

	wg.Wait()
}

func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}
