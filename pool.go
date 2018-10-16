// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/mmatczuk/go-http-tunnel/id"
)

const connKey = "conn"

type clientConnections struct {
	first  *clientConnection
	last   *clientConnection
	count  int
	mu     sync.Mutex
	lastid int
	conns  []*clientConnection
}

func (cs *clientConnections) low() *clientConnection {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	sort.Slice(cs.conns, func(i, j int) bool {
		return cs.conns[i].count < cs.conns[j].count
	})
	return cs.conns[0].increase()
}

func (cs *clientConnections) add(con *http2.ClientConn) *clientConnection {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	c := &clientConnection{con, cs.last, nil, cs.lastid, 0}
	cs.lastid++
	if cs.last == nil {
		cs.first = c
	} else {
		c.prev.next = c
	}
	cs.last = c
	cs.count++
	cs.conns = append(cs.conns, c)
	return cs.last
}

func (cs *clientConnections) remove(c *clientConnection) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.count == 1 {
		cs.first = nil
		cs.last = nil
	} else if cs.last == c {
		cs.last = c.prev
		cs.last.next = nil
	} else if cs.first == c {
		cs.first = c.next
		cs.first.prev = nil
	} else {
		p, n := c.prev, c.next
		p.next = n
		n.prev = p
	}

	c.prev = nil
	c.next = nil
	cs.count--
}

type clientConnectionSetter struct {
	conn *clientConnection
}

type clientConnection struct {
	*http2.ClientConn
	prev  *clientConnection
	next  *clientConnection
	id    int
	count int
}

func (c *clientConnection) decrease() *clientConnection {
	c.count--
	return c
}

func (c *clientConnection) increase() *clientConnection {
	c.count++
	return c
}

type connPair struct {
	controller *clientConnectionController
	clientConn *clientConnection
	conns      *clientConnections
	current    *clientConnection
	mu         sync.Mutex
}

func (cp *connPair) next() *clientConnection {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	con := cp.conns.low()
	cp.current = con
	return con
}

type connPool struct {
	t     *http2.Transport
	conns map[string]*connPair // key is host:port
	free  func(identifier id.ID)
	mu    sync.RWMutex
}

func newConnPool(t *http2.Transport, f func(identifier id.ID)) *connPool {
	return &connPool{
		t:     t,
		free:  f,
		conns: make(map[string]*connPair),
	}
}
func (p *connPool) newRequest(method string, identifier id.ID, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, p.URL(identifier), body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(context.WithValue(req.Context(), connKey, &clientConnectionSetter{}))
	return req, err
}
func (p *connPool) URL(identifier id.ID) string {
	return fmt.Sprint("https://", identifier)
}

func (p *connPool) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if cp, ok := p.conns[addr]; ok {
		conn := cp.next()
		if setter := req.Context().Value(connKey); setter != nil {
			setter.(*clientConnectionSetter).conn = conn
		}
		cp.controller.logger.Log("level", 3, "client_conn", fmt.Sprintf("#%d", conn.id), "addr", addr)
		return conn.ClientConn, nil
	}

	return nil, errClientNotConnected
}

func (p *connPool) MarkDead(c *http2.ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for addr, cp := range p.conns {
		if cp.clientConn.ClientConn == c {
			p.close(cp, addr)
			return
		}
	}
}

func (p *connPool) Has(ID id.ID) (ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(ID)
	_, ok = p.conns[addr]
	return
}

func (p *connPool) AddClientConnection(ID id.ID, conn net.Conn) (ccon *clientConnection, err error) {
	c, err := p.t.NewClientConn(conn)
	if err != nil {
		return nil, err
	}

	addr := p.addr(ID)

	cp := p.conns[addr]
	if (cp.conns.count - 1) >= cp.controller.cfg.Connections {
		return nil, errClientManyConnections
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return cp.conns.add(c), nil
}

func (p *connPool) AddConn(controller *clientConnectionController) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(controller.ID)

	if cp, ok := p.conns[addr]; ok {
		if err := p.ping(cp); err != nil {
			p.close(cp, addr)
		} else {
			return errClientAlreadyConnected
		}
	}

	c, err := p.t.NewClientConn(controller.conn)
	if err != nil {
		return err
	}
	cp := &connPair{controller: controller, conns: &clientConnections{}}
	cp.clientConn = cp.conns.add(c)
	p.conns[addr] = cp

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

func (p *connPool) closeReqConn(req *http.Request) {
	if conn := req.Context().Value(connKey); conn != nil {
		conn.(*clientConnectionSetter).conn.decrease()
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

func (p *connPool) ping(cp *connPair) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
	defer cancel()

	return cp.clientConn.Ping(ctx)
}

func (p *connPool) close(cp *connPair, addr string) {
	cp.controller.conn.Close()
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
