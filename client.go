package h2tun

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/koding/h2tun/proto"
	"github.com/koding/logging"
	"golang.org/x/net/http2"
)

type Client struct {
	proxy      ProxyFunc
	serverAddr string
	tlsConfig  *tls.Config
	conn       net.Conn
	httpServer *http2.Server
	log        logging.Logger
}

func NewClient(proxy ProxyFunc, serverAddr string, tlsConfig *tls.Config) *Client {
	return &Client{
		proxy:      proxy,
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
		Handler: http.HandlerFunc(c.serveHTTP),
	})

	return nil
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
	c.log.Debug("Proxy init %s %v", r.RemoteAddr, msg)
	c.proxy(flushWriter{w}, r.Body, msg)
	c.log.Debug("Proxy over %s %v", r.RemoteAddr, msg)
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

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
