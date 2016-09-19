package h2tun

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/koding/logging"
	"golang.org/x/net/http2"
)

type Client struct {
	serverAddr string
	tlsConfig  *tls.Config
	conn       net.Conn
	httpServer *http2.Server
	log        logging.Logger
}

func NewClient(serverAddr string, tlsConfig *tls.Config) *Client {
	return &Client{
		serverAddr: serverAddr,
		tlsConfig:  tlsConfig,
		httpServer: &http2.Server{},
		log:        logging.NewLogger("client"),
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

type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

func (c *Client) proxy(w http.ResponseWriter, r *http.Request) {
	c.log.Info("New proxy request")
	if r.Method == http.MethodConnect {
		http.Error(w, "OK", http.StatusOK)
	} else {
		io.Copy(flushWriter{w}, r.Body)
	}
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
