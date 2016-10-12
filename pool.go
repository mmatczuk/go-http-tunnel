package h2tun

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
)

var (
	errNoClientConn           = errors.New("no connection")
	errClientAlreadyConnected = errors.New("client already connected")
)

type connPool struct {
	t     *http2.Transport
	mu    sync.RWMutex
	conns map[string]*http2.ClientConn // key is host:port
}

func newConnPool(t *http2.Transport) *connPool {
	return &connPool{
		t:     t,
		conns: make(map[string]*http2.ClientConn),
	}
}

func (p *connPool) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if cc, ok := p.conns[addr]; ok && cc.CanTakeNewRequest() {
		return cc, nil
	}

	return nil, errNoClientConn
}

func (p *connPool) MarkDead(c *http2.ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, v := range p.conns {
		if v == c {
			delete(p.conns, k)
		}
		break
	}
}

func (p *connPool) addHostConn(host string, conn net.Conn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.conns[host]; ok {
		return errClientAlreadyConnected
	}

	cc, err := p.t.NewClientConn(conn)
	if err != nil {
		return err
	}

	p.conns[hostPort(host)] = cc

	return nil
}

func (p *connPool) markHostDead(host string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.conns, hostPort(host))
}

func hostPort(host string) string {
	return fmt.Sprint(host, ":443")
}
