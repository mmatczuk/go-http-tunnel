package h2tun

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/andrew-d/id"
	"github.com/koding/logging"
)

type Server struct {
	allowedClients []*AllowedClient
	listener       net.Listener

	httpClient *http.Client
	hostConn   map[string]net.Conn
	hostConnMu sync.RWMutex

	log logging.Logger
}

type AllowedClient struct {
	ID   id.ID
	Host string
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

	go s.listenControl()

	return s, nil
}

func (s *Server) initHTTPClient() {
	s.hostConn = make(map[string]net.Conn)

	s.httpClient = &http.Client{
		// TODO mma try using connection pool for transport
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

	id, err := peerID(conn.(*tls.Conn))
	if err != nil {
		s.log.Warning("Certificate error: %s", err)
		conn.Close()
		return
	}
	client, ok := s.checkID(id)
	if !ok {
		s.log.Warning("Unknown certificate: %q", id.String())
		conn.Close()
		return
	}

	req, err := http.NewRequest(http.MethodConnect, fmt.Sprintf("https://%s", client.Host), nil)
	if err != nil {
		s.log.Error("Invalid host %q for client %q", client.Host, client.ID)
		conn.Close()
		return
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		s.log.Warning("Setting no deadline failed: %s", err)
	}

	s.addHostConn(client, conn)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.log.Warning("Handshake failed %s", err)
		conn.Close()
		s.deleteHostConn(client.Host)
		return
	}
	if resp.StatusCode != http.StatusOK {
		s.log.Warning("Handshake failed")
		conn.Close()
		s.deleteHostConn(client.Host)
		return
	}
}

func (s *Server) addHostConn(client *AllowedClient, conn net.Conn) {
	key := hostPort(client.Host)

	s.hostConnMu.Lock()
	oldConn := s.hostConn[key]
	if oldConn != nil {
		s.log.Info("Closing old connection for host &q, old was from %s, new is from %s",
			client.Host, oldConn.RemoteAddr().String(), conn.RemoteAddr().String())
		oldConn.Close()
	}
	s.hostConn[key] = conn
	s.hostConnMu.Unlock()
}

func (s *Server) deleteHostConn(host string) {
	key := hostPort(host)
	s.hostConnMu.Lock()
	delete(s.hostConn, key)
	s.hostConnMu.Unlock()
}

func hostPort(host string) string {
	// TODO add support for custom ports
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

func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}
