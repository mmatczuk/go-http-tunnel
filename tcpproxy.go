package tunnel

import (
	"fmt"
	"io"
	"net"

	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// TCPProxy forwards TCP streams.
type TCPProxy struct {
	// localAddr specifies default TCP address of the local server.
	localAddr string
	// localAddrMap specifies mapping from ControlMessage ForwardedBy to
	// local server address, keys may contain host and port, only host or
	// only port. The order of precedence is the following
	// * host and port
	// * port
	// * host
	localAddrMap map[string]string
	// logger is the proxy logger.
	logger log.Logger
}

// NewTCPProxy creates new direct TCPProxy, everything will be proxied to
// localAddr.
func NewTCPProxy(localAddr string, logger log.Logger) *TCPProxy {
	if localAddr == "" {
		panic("Empty localAddr")
	}

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
	if localAddrMap == nil {
		panic("Empty localAddrMap")
	}

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
	w = flushWriter{w}

	target := p.localAddrFor(msg.ForwardedBy)
	if target == "" {
		p.logger.Log(
			"level", 1,
			"msg", "no target",
			"host", msg.ForwardedBy,
		)
		return
	}

	local, err := net.DialTimeout("tcp", target, DefaultTimeout)
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "dial failed",
			"target", target,
			"err", err,
		)
		return
	}

	done := make(chan struct{})
	go func() {
		transfer(w, local, log.NewContext(p.logger).With(
			"dst", msg.ForwardedBy,
			"src", target,
		))
		close(done)
	}()

	transfer(local, r, log.NewContext(p.logger).With(
		"dst", target,
		"src", msg.ForwardedBy,
	))

	<-done
}

func (p *TCPProxy) localAddrFor(hostPort string) string {
	if p.localAddrMap == nil {
		return p.localAddr
	}

	// try hostPort
	if addr := p.localAddrMap[hostPort]; addr != "" {
		return addr
	}

	// try port
	host, port, _ := net.SplitHostPort(hostPort)
	if addr := p.localAddrMap[port]; addr != "" {
		return addr
	}

	// try 0.0.0.0:port
	if addr := p.localAddrMap[fmt.Sprintf("0.0.0.0:%s", port)]; addr != "" {
		return addr
	}

	// try host
	if addr := p.localAddrMap[host]; addr != "" {
		return addr
	}

	return p.localAddr
}
