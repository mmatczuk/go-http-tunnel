package tunnel

import (
	"fmt"
	"io"
	"net/url"

	"golang.org/x/net/websocket"

	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// WSProxy forwards HTTP traffic.
type WSProxy struct {
	// localURL specifies default base URL of local service.
	localURL *url.URL
	// logger is the proxy logger.
	logger log.Logger
}

// NewWSProxy creates a new direct WSProxy, everything will be proxied to
// localURL.
func NewWSProxy(localURL *url.URL, logger log.Logger) *WSProxy {
	if localURL == nil {
		panic("Empty localURL")
	}

	if logger == nil {
		logger = log.NewNopLogger()
	}

	p := &WSProxy{
		localURL: localURL,
		logger:   logger,
	}

	return p
}

// Proxy is a ProxyFunc.
func (p *WSProxy) Proxy(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	w = flushWriter{w}

	target := p.localURL
	if target == nil {
		p.logger.Log(
			"level", 1,
			"msg", "no target",
			"host", msg.ForwardedBy,
		)
		return
	}
	// TODO support path target.Path = singleJoiningSlash(target.Path, msg.Path)

	config, err := websocket.NewConfig(target.String(), fmt.Sprintf("http://%s/", msg.ForwardedBy))
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "failed to create ws config",
			"err", err,
		)
		return
	}
	// TODO support config.Header

	ws, err := websocket.DialConfig(config)
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "ws dial failed",
			"err", err,
		)
		return
	}

	done := make(chan struct{})
	go func() {
		transfer(w, ws, log.NewContext(p.logger).With(
			"dst", msg.ForwardedBy,
			"src", target,
		))
		close(done)
	}()

	transfer(ws, r, log.NewContext(p.logger).With(
		"dst", target,
		"src", msg.ForwardedBy,
	))

	<-done
}
