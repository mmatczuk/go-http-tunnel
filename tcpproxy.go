package tunnel

import (
	"fmt"
	"io"
	"net"

	"github.com/koding/logging"
	"github.com/mmatczuk/tunnel/proto"
)

var (
	tpcLog = logging.NewLogger("tcp")
)

// TCPProxy forwards TCP streams.
type TCPProxy struct {
	// LocalAddr defines TCP address of the local server.
	LocalAddr string
	// LocalAddrMap specifies a mapping from ControlMessage.ForwardedBy port
	// to local server. If port is not found then if LocalAddr is not empty
	// it will be used as a default otherwise connection will be closed.
	LocalAddrMap map[string]string
	// Log specifies the logger. If nil a default logging.Logger is used.
	Log logging.Logger
}

// Proxy is a ProxyFunc.
func (p *TCPProxy) Proxy(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	if msg.Protocol != "tcp" {
		panic(fmt.Sprintf("Expected proxy protocol, got %s", msg.Protocol))
	}

	var log = p.log()

	_, port, err := net.SplitHostPort(msg.ForwardedBy)
	if err != nil {
		log.Error("Failed to parse input address: %s", err)
		return
	}

	localAddr := p.localAddr(port)
	if localAddr == "" {
		log.Warning("Failed to get local address for port %q", port)
		return
	}

	log.Debug("Dialing local server: %q", localAddr)
	local, err := net.DialTimeout("tcp", localAddr, DefaultDialTimeout)
	if err != nil {
		log.Error("Dialing local server %q failed: %s", localAddr, err)
		return
	}

	done := make(chan struct{})
	go func() {
		transfer("local to remote", local, r)
		close(done)
	}()
	transfer("remote to local", w, local)
}

func (p *TCPProxy) localAddr(port string) string {
	if p.LocalAddrMap == nil {
		return p.LocalAddr
	}

	addr, ok := p.LocalAddrMap[port]
	if !ok {
		return p.LocalAddr
	}

	return addr
}

func (p *TCPProxy) log() logging.Logger {
	if p.Log != nil {
		return p.Log
	}
	return tpcLog
}
