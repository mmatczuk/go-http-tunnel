package tunnel

import (
	"io"

	"github.com/mmatczuk/go-http-tunnel/proto"
)

// ProxyFunc is responsible for forwarding a remote connection to local server
// and writing the response.
type ProxyFunc func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage)

// ProxyFuncs is a collection of ProxyFunc.
type ProxyFuncs struct {
	// HTTP is custom implementation of HTTP proxing.
	HTTP ProxyFunc
	// WS is custom implementation of WS proxing.
	WS ProxyFunc
	// TCP is custom implementation of TCP proxing.
	TCP ProxyFunc
}

// Proxy returns a ProxyFunc that uses custom function if provided.
func Proxy(p ProxyFuncs) ProxyFunc {
	return func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
		var f ProxyFunc
		switch msg.Protocol {
		case proto.HTTP:
			f = p.HTTP
		case proto.WS:
			f = p.WS
		case proto.TCP, proto.TCP4, proto.TCP6, proto.UNIX:
			f = p.TCP
		}

		if f == nil {
			return
		}

		f(w, r, msg)
	}
}
