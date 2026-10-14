// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// TCPProxy forwards TCP streams.
type TCPProxy struct {
	// localAddr specifies default TCP address of the local server.
	localAddr string
	// localAddrMap specifies mapping from ControlMessage.ForwardedHost to
	// local server address, keys may contain host and port, only host or
	// only port. The order of precedence is the following
	// * host and port
	// * port
	// * host
	// * wildcard domain (*.example.com)
	// * catch-all (*)
	localAddrMap map[string]string
	// ForwardProxy enables forward proxying directly to the destination when no
	// local address is mapped.
	ForwardProxy bool
	// logger is the proxy logger.
	logger log.Logger
}

// NewTCPProxy creates new direct TCPProxy, everything will be proxied to
// localAddr.
func NewTCPProxy(localAddr string, logger log.Logger) *TCPProxy {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &TCPProxy{
		localAddr: localAddr,
		logger:    logger,
	}
}

// NewMultiTCPProxy creates a new dispatching TCPProxy, connections may go to
// different backends based on localAddrMap.
func NewMultiTCPProxy(localAddrMap map[string]string, logger log.Logger) *TCPProxy {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &TCPProxy{
		localAddrMap: localAddrMap,
		logger:       logger,
	}
}

// Proxy is a ProxyFunc.
func (p *TCPProxy) Proxy(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	switch msg.ForwardedProto {
	case proto.TCP, proto.TCP4, proto.TCP6, proto.UNIX, proto.SNI:
		// ok
	default:
		p.logger.Log(
			"level", 0,
			"msg", "unsupported protocol",
			"ctrlMsg", msg,
		)
		return
	}

	target := p.localAddrFor(msg.ForwardedHost)
	if target == "" {
		if p.ForwardProxy || p.hasWildcard() {
			target = msg.ForwardedHost
		} else {
			p.logger.Log(
				"level", 1,
				"msg", "no target",
				"ctrlMsg", msg,
			)
			return
		}
	}

	local, err := net.DialTimeout("tcp", target, DefaultTimeout)
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "dial failed",
			"target", target,
			"ctrlMsg", msg,
			"err", err,
		)
		return
	}
	defer local.Close()

	if err := keepAlive(local); err != nil {
		p.logger.Log(
			"level", 1,
			"msg", "TCP keepalive for tunneled connection failed",
			"target", target,
			"ctrlMsg", msg,
			"err", err,
		)
	}

	done := make(chan struct{})
	go func() {
		transfer(flushWriter{w}, local, log.NewContext(p.logger).With(
			"dst", msg.ForwardedHost,
			"src", target,
		))
		close(done)
	}()

	transfer(local, r, log.NewContext(p.logger).With(
		"dst", target,
		"src", msg.ForwardedHost,
	))

	<-done
}

func (p *TCPProxy) hasWildcard() bool {
	if _, ok := p.localAddrMap["*"]; ok {
		return true
	}
	for k := range p.localAddrMap {
		if strings.HasPrefix(k, "*.") {
			return true
		}
	}
	return false
}

func (p *TCPProxy) localAddrFor(hostPort string) string {
	if len(p.localAddrMap) == 0 {
		return p.localAddr
	}

	// try hostPort
	if addr := p.localAddrMap[hostPort]; addr != "" {
		return addr
	}

	// try port
	host, port, _ := net.SplitHostPort(hostPort)
	if host == "" {
		host = hostPort
	}
	if port != "" {
		if addr := p.localAddrMap[port]; addr != "" {
			return addr
		}

		// try 0.0.0.0:port
		if addr := p.localAddrMap[fmt.Sprintf("0.0.0.0:%s", port)]; addr != "" {
			return addr
		}
	}

	// try host
	if addr := p.localAddrMap[host]; addr != "" {
		return addr
	}

	// try wildcard domain match (e.g. *.example.com)
	if dot := strings.Index(host, "."); dot != -1 {
		for dot != -1 {
			wildcard := "*" + host[dot:]
			if addr := p.localAddrMap[wildcard]; addr != "" {
				return addr
			}
			nextDot := strings.Index(host[dot+1:], ".")
			if nextDot == -1 {
				break
			}
			dot += 1 + nextDot
		}
	}

	// try "*"
	if addr := p.localAddrMap["*"]; addr != "" {
		return addr
	}

	return p.localAddr
}
