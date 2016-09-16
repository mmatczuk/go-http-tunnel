package h2tun

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"golang.org/x/net/http2"
)

type Client struct {
	serverAddr string
	tlsConfig  *tls.Config
	conn       net.Conn
	httpServer *http2.Server
}

func NewClient(serverAddr string, tlsConfig *tls.Config) *Client {
	return &Client{
		serverAddr: serverAddr,
		tlsConfig:  tlsConfig,
		httpServer: &http2.Server{},
	}
}

func (c *Client) Connect() error {
	conn, err := tls.Dial("tcp", c.serverAddr, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %s", err)
	}
	c.conn = conn

	c.httpServer.ServeConn(conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(c.proxy),
	})

	return nil
}

func (c *Client) proxy(w http.ResponseWriter, r *http.Request) {
	fmt.Print(r.URL)
	http.Error(w, "OK", http.StatusOK)
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
