package tunnel

import (
	"fmt"
	"io"
	"net"

	"github.com/koding/logging"
	"github.com/mmatczuk/tunnel/proto"
)

// TCPProxy forwards TCP streams.
type TCPProxy struct {
	// Log is the proxy logger.
	Log logging.Logger
	// localAddr specifies default TCP address of the local server.
	localAddr string
	// localAddrMap specifies mapping from ControlMessage ForwardedBy to
	// local server address, keys may contain host and port, only host or
	// only port. The order of precedence is the following
	// * host and port
	// * port
	// * host
	localAddrMap map[string]string
}

// NewTCPProxy creates new direct TCPProxy, everything will be proxied to
// localAddr.
func NewTCPProxy(localAddr string) *TCPProxy {
	return &TCPProxy{
		Log:       logging.NewLogger("tcpproxy"),
		localAddr: localAddr,
	}
}

// NewMultiTCPProxy creates a new dispatching TCPProxy, connections may go to
// different backends based on localAddrMap, see TCPProxy localAddrMap docs for
// more details.
func NewMultiTCPProxy(localAddrMap map[string]string) *TCPProxy {
	return &TCPProxy{
		Log:          logging.NewLogger("tcpproxy"),
		localAddrMap: localAddrMap,
	}
}

// Proxy is a ProxyFunc.
func (p *TCPProxy) Proxy(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	w = flushWriter{w}

	if msg.Protocol != "tcp" {
		panic(fmt.Sprintf("Expected proxy protocol, got %s", msg.Protocol))
	}

	target := p.localAddrFor(msg.ForwardedBy)
	if target == "" {
		p.Log.Warning("Failed to get local address")
		return
	}

	local, err := net.DialTimeout("tcp", target, DefaultDialTimeout)
	if err != nil {
		p.Log.Error("Dialing local server %q failed: %s", target, err)
		return
	}

	done := make(chan struct{})
	go func() {
		transfer("local to remote", w, local)
		close(done)
	}()
	transfer("remote to local", local, r)
}

func (p *TCPProxy) localAddrFor(hostPort string) string {
	if p.localAddrMap == nil {
		return p.localAddr
	}

	// try host and port
	if addr := p.localAddrMap[hostPort]; addr != "" {
		return addr
	}

	// try port
	host, port, _ := net.SplitHostPort(hostPort)
	if addr := p.localAddrMap[port]; addr != "" {
		return addr
	}

	// try host
	if addr := p.localAddrMap[host]; addr != "" {
		return addr
	}

	return p.localAddr
}
