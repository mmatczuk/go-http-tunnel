// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/mmatczuk/go-http-tunnel/id"
)

type connPair struct {
	conn       net.Conn
	clientConn *http2.ClientConn
}

type connPool struct {
	t     *http2.Transport
	conns map[string]connPair // key is host:port
	free  func(identifier id.ID)
	mu    sync.RWMutex
}

func newConnPool(t *http2.Transport, f func(identifier id.ID)) *connPool {
	return &connPool{
		t:     t,
		free:  f,
		conns: make(map[string]connPair),
	}
}

func (p *connPool) URL(identifier id.ID) string {
	return fmt.Sprint("https://", identifier)
}

func (p *connPool) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if cp, ok := p.conns[addr]; ok && cp.clientConn.CanTakeNewRequest() {
		return cp.clientConn, nil
	}

	return nil, errClientNotConnected
}

func (p *connPool) MarkDead(c *http2.ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for addr, cp := range p.conns {
		if cp.clientConn == c {
			p.close(cp, addr)
			return
		}
	}
}

func (p *connPool) AddConn(conn net.Conn, identifier id.ID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(identifier)

	if cp, ok := p.conns[addr]; ok {
		if err := p.ping(cp); err != nil {
			p.close(cp, addr)
		} else {
			return errClientAlreadyConnected
		}
	}

	c, err := p.t.NewClientConn(conn)
	if err != nil {
		return err
	}
	p.conns[addr] = connPair{
		conn:       conn,
		clientConn: c,
	}

	return nil
}

func (p *connPool) DeleteConn(identifier id.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(identifier)

	if cp, ok := p.conns[addr]; ok {
		p.close(cp, addr)
	}
}

func (p *connPool) Ping(identifier id.ID) (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(identifier)

	if cp, ok := p.conns[addr]; ok {
		start := time.Now()
		err := p.ping(cp)
		return time.Since(start), err
	}

	return 0, errClientNotConnected
}

func (p *connPool) ping(cp connPair) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
	defer cancel()

	return cp.clientConn.Ping(ctx)
}

func (p *connPool) close(cp connPair, addr string) {
	cp.conn.Close()
	delete(p.conns, addr)
	if p.free != nil {
		p.free(p.identifier(addr))
	}
}

func (p *connPool) addr(identifier id.ID) string {
	return fmt.Sprint(identifier.String(), ":443")
}

func (p *connPool) identifier(addr string) id.ID {
	var identifier id.ID
	identifier.UnmarshalText([]byte(addr[:len(addr)-4]))
	return identifier
}
