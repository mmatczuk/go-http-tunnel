package h2tun

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/koding/h2tun/proto"
	"github.com/koding/logging"
	"golang.org/x/net/http2"
)

// ClientConfig is Client configuration object.
type ClientConfig struct {
	// ServerAddr specifies TCP address of the tunnel server.
	ServerAddr string
	// TLSClientConfig specifies the TLS configuration to use with tls.Client.
	TLSClientConfig *tls.Config
	// DialTLS specifies an optional dial function for creating
	// TLS connections for requests.
	//
	// If DialTLS is nil, tls.Dial is used.
	DialTLS func(network, addr string, config *tls.Config) (net.Conn, error)
	// Proxy specifies proxying rules.
	Proxy ProxyFunc
	// Log specifies the logger. If nil a default logging.Logger is used.
	Log logging.Logger
}

// Client is client to tunnel server.
type Client struct {
	config     *ClientConfig
	conn       net.Conn
	httpServer *http2.Server
	log        logging.Logger
}

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

func (c *Client) Connect() error {
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

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
