package h2tun

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/koding/logging"
	"github.com/mmatczuk/h2tun/proto"
	"golang.org/x/net/http2"
)

// ClientConfig defines configuration for the Client.
type ClientConfig struct {
	// ServerAddr specifies TCP address of the tunnel server.
	ServerAddr string
	// TLSClientConfig specifies the tls configuration to use with tls.Client.
	TLSClientConfig *tls.Config
	// DialTLS specifies an optional dial function that creates a tls
	// connection to the server. If DialTLS is nil, tls.Dial is used.
	DialTLS func(network, addr string, config *tls.Config) (net.Conn, error)
	// Proxy is ProxyFunc responsible for transferring data between server
	// and local services.
	Proxy ProxyFunc
	// Log specifies the logger. If nil a default logging.Logger is used.
	Log logging.Logger
}

// Client is responsible for creating connection to the server, handling control
// messages. It uses ProxyFunc for transferring data between server and local
// services.
type Client struct {
	config     *ClientConfig
	conn       net.Conn
	connMu     sync.Mutex
	httpServer *http2.Server
	log        logging.Logger
}

// NewClient creates a new unconnected Client based on configuration. Caller
// must invoke Start() on returned instance in order to connect server.
func NewClient(config *ClientConfig) *Client {
	log := logging.NewLogger("client")
	if config.Log != nil {
		log = config.Log
	}

	c := &Client{
		config:     config,
		httpServer: &http2.Server{},
		log:        log,
	}

	return c
}

// Start connects client to the server, it returns error if there is a dial error,
// otherwise it spawns a new goroutine with http/2 server handling ControlMessages.
func (c *Client) Start() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	c.log.Info("Connecting to %q", c.config.ServerAddr)
	conn, err := c.dial("tcp", c.config.ServerAddr, c.config.TLSClientConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %s", err)
	}
	c.conn = conn

	go c.httpServer.ServeConn(conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(c.serveHTTP),
	})

	return nil
}

func (c *Client) dial(network, addr string, config *tls.Config) (net.Conn, error) {
	if c.config.DialTLS != nil {
		return c.config.DialTLS(network, addr, config)
	}
	return tls.Dial(network, addr, config)
}

func (c *Client) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		c.log.Info("Handshake: hello from server")
		http.Error(w, "Nice to see you", http.StatusOK)
		return
	}

	msg, err := proto.ParseControlMessage(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c.log.Debug("Start proxying %v", msg)
	c.config.Proxy(flushWriter{w}, r.Body, msg)
	c.log.Debug("Done proxying %v", msg)
}

// Stop closes the connection between client and server. After stopping client
// can be started again.
func (c *Client) Stop() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return nil
	}
	c.httpServer = nil
	return c.conn.Close()
}
