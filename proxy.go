// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

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
	// TCP is custom implementation of TCP proxing.
	TCP ProxyFunc
}

// Proxy returns a ProxyFunc that uses custom function if provided.
func Proxy(p ProxyFuncs) ProxyFunc {
	return func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
		var f ProxyFunc
		switch msg.ForwardedProto {
		case proto.HTTP, proto.HTTPS:
			f = p.HTTP
		case proto.TCP, proto.TCP4, proto.TCP6, proto.UNIX:
			f = p.TCP
		}

		if f == nil {
			return
		}

		f(w, r, msg)
	}
}
