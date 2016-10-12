package h2tun

import (
	"io"

	"github.com/mmatczuk/h2tun/proto"
)

// ProxyFunc is responsible for forwarding a remote connection to local server
// and writing the response.
type ProxyFunc func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage)
