// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"strconv"

	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// ClientConfig is configuration of the Client.
type ClientConfig struct {
	// ServerAddr specifies TCP address of the tunnel server.
	ServerAddr string
	// TLSClientConfig specifies the tls configuration to use with
	// tls.Client.
	TLSClientConfig *tls.Config
	// DialTLS specifies an optional dial function that creates a tls
	// connection to the server. If DialTLS is nil, tls.Dial is used.
	DialTLS func(network, addr string, config *tls.Config) (net.Conn, error)
	// Backoff specifies backoff policy on server connection retry. If nil
	// when dial fails it will not be retried.
	Backoff Backoff
	// Tunnels specifies the tunnels controller requests to be opened on server.
	Tunnels map[string]*proto.Tunnel
	// Proxy is ProxyFunc responsible for transferring data between server
	// and local services.
	Proxy ProxyFunc
	// Logger is optional logger. If nil logging is disabled.
	Logger log.Logger
	// Name the controller name.
	Name string
	// Description Client Description.
	Description string
	// Registered if client is registered on server.
	Registered bool
}

type ClientToServerConn struct {
	c              *Client
	conn           net.Conn
	connMu         sync.Mutex
	httpServer     *http2.Server
	serverErr      error
	lastDisconnect time.Time
	logger         log.Logger
	handler        http.HandlerFunc
}

// Client is responsible for creating connection to the server, handling control
// messages. It uses ProxyFunc for transferring data between server and local
// services.
type Client struct {
	*ClientToServerConn
	config          *ClientConfig
	connectionCount int
	connections     []*ClientToServerConn
	hostname        string
}

// NewClient creates a new unconnected Client based on configuration. Caller
// must invoke Start() on returned instance in order to connect server.
func NewClient(config *ClientConfig) (*Client, error) {
	if config.ServerAddr == "" {
		return nil, errors.New("missing ServerAddr")
	}
	if config.TLSClientConfig == nil {
		return nil, errors.New("missing TLSClientConfig")
	}
	if len(config.Tunnels) == 0 {
		return nil, errors.New("missing Tunnels")
	}
	if config.Proxy == nil {
		return nil, errors.New("missing Proxy")
	}

	logger := config.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}

	c := &Client{
		config: config,
	}

	c.hostname, _ = os.Hostname()

	c.ClientToServerConn = &ClientToServerConn{
		c:          c,
		httpServer: &http2.Server{},
		logger:     logger,
		handler: func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				if r.Header.Get(proto.HeaderError) != "" {
					c.handleHandshakeError(w, r)
				} else {
					c.handleHandshake(w, r)
				}
				return
			}
			c.ClientToServerConn.serveHTTP(w, r)
		}}

	return c, nil
}

// Hostname returns the client hostname
func (c *Client) Hostname() string {
	return c.hostname
}

// Start connects controller to the server, it returns error if there is a
// connection error, or server cannot open requested tunnels. On connection
// error a backoff policy is used to reestablish the connection. When connected
// HTTP/2 server is started to handle ControlMessages.
func (c *ClientToServerConn) Start() error {
	c.logger.Log(
		"level", 1,
		"action", "start",
	)

	for {
		conn, err := c.connect()
		if err != nil {
			return err
		}

		handler := c.handler

		if handler == nil {
			handler = http.HandlerFunc(c.serveHTTP)
		}

		c.httpServer.ServeConn(conn, &http2.ServeConnOpts{
			Handler: handler,
		})

		c.logger.Log(
			"level", 1,
			"action", "disconnected",
		)

		c.connMu.Lock()
		now := time.Now()
		err = c.serverErr

		// detect disconnect hiccup
		if err == nil && now.Sub(c.lastDisconnect).Seconds() < 5 {
			err = fmt.Errorf("connection is being cut")
		}

		c.conn = nil
		c.serverErr = nil
		c.lastDisconnect = now
		c.connMu.Unlock()

		if err != nil {
			return err
		}
	}
}

func (c *ClientToServerConn) connect() (net.Conn, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return nil, fmt.Errorf("already connected")
	}

	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %s", err)
	}
	c.conn = conn

	return conn, nil
}

func (c *ClientToServerConn) dial() (net.Conn, error) {
	var (
		network   = "tcp"
		addr      = c.c.config.ServerAddr
		tlsConfig = c.c.config.TLSClientConfig
	)

	doDial := func() (conn net.Conn, err error) {
		c.logger.Log(
			"level", 1,
			"action", "dial",
			"network", network,
			"addr", addr,
		)

		if c.c.config.DialTLS != nil {
			conn, err = c.c.config.DialTLS(network, addr, tlsConfig)
		} else {
			d := &net.Dialer{
				Timeout: DefaultTimeout,
			}
			conn, err = d.Dial(network, addr)

			if err == nil {
				err = keepAlive(conn)
			}
			if err == nil {
				conn = tls.Client(conn, tlsConfig)
			}
			if err == nil {
				err = conn.(*tls.Conn).Handshake()
			}
		}

		if err != nil {
			if conn != nil {
				conn.Close()
				conn = nil
			}

			c.logger.Log(
				"level", 0,
				"msg", "dial failed",
				"network", network,
				"addr", addr,
				"err", err,
			)
		}

		return
	}

	b := c.c.config.Backoff
	if b == nil {
		return doDial()
	}

	for {
		conn, err := doDial()

		// success
		if err == nil {
			b.Reset()
			return conn, err
		}

		// failure
		d := b.NextBackOff()
		if d < 0 {
			return conn, fmt.Errorf("backoff limit exeded: %s", err)
		}

		// backoff
		c.logger.Log(
			"level", 1,
			"action", "backoff",
			"sleep", d,
		)
		time.Sleep(d)
	}
}

func (c *ClientToServerConn) serveHTTP(w http.ResponseWriter, r *http.Request) {
	msg, err := proto.ReadControlMessage(r)
	if err != nil {
		c.logger.Log(
			"level", 1,
			"err", err,
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.logger.Log(
		"level", 2,
		"action", "handle",
		"ctrlMsg", msg,
	)
	switch msg.Action {
	case proto.ActionProxy:
		c.c.config.Proxy(w, r.Body, msg)
	default:
		c.logger.Log(
			"level", 0,
			"msg", "unknown action",
			"ctrlMsg", msg,
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	c.logger.Log(
		"level", 2,
		"action", "done",
		"ctrlMsg", msg,
	)
}

func (c *Client) handleHandshakeError(w http.ResponseWriter, r *http.Request) {
	err := fmt.Errorf(r.Header.Get(proto.HeaderError))

	c.logger.Log(
		"level", 1,
		"action", "handshake error",
		"addr", r.RemoteAddr,
		"err", err,
	)

	c.connMu.Lock()
	c.serverErr = fmt.Errorf("server error: %s", err)
	c.connMu.Unlock()
}

func (c *Client) handleHandshake(w http.ResponseWriter, r *http.Request) {
	c.logger.Log(
		"level", 1,
		"action", "handshake",
		"addr", r.RemoteAddr,
	)

	w.WriteHeader(http.StatusOK)

	var info interface{}

	if c.config.Registered {
		if cc := r.Header.Get(HeaderConnectionsCount); cc != "" {
			c.connectionCount, _ = strconv.Atoi(cc)
		}

		tunnels := map[string]*proto.RegisteredTunnel{}
		for name, t := range c.config.Tunnels {
			tunnels[name] = &proto.RegisteredTunnel{
				Disabled:  t.Disabled,
				LocalAddr: t.LocalAddr,
				Auth:      t.Auth,
			}
		}

		info = &RegisteredClientInfo{
			Hostname: c.hostname,
			Tunnels:  tunnels,
		}
	} else {
		info = &RegisteredClientConfig{
			Name:        c.config.Name,
			Description: c.config.Description,
			Tunnels:     c.config.Tunnels,
			Hostname:    c.hostname,
		}
	}

	b, err := json.Marshal(info)

	if err != nil {
		c.logger.Log(
			"level", 0,
			"msg", "handshake failed",
			"err", err,
		)
		return
	}

	if _, err := w.Write(b); err == nil && c.config.Registered && c.connectionCount > 0 {
		if len(c.connections) == 0 {
			for i := 0; i < c.connectionCount; i++ {
				c.connections = append(c.connections, &ClientToServerConn{
					c:          c,
					httpServer: c.httpServer,
					logger:     log.NewContext(c.logger).With(fmt.Sprint("[", i, "]")),
				})
			}

			for _, cc := range c.connections {
				go cc.Start()
			}
		}
	}
}

// Stop disconnects controller from server.
func (c *ClientToServerConn) Stop() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	c.logger.Log(
		"level", 1,
		"action", "stop",
	)

	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = nil
}
