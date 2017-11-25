// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"

	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// HTTPProxy forwards HTTP traffic.
type HTTPProxy struct {
	httputil.ReverseProxy
	// localURL specifies default base URL of local service.
	localURL *url.URL
	// localURLMap specifies mapping from ControlMessage.ForwardedHost to
	// local service URL, keys may contain host and port, only host or
	// only port. The order of precedence is the following
	// * host and port
	// * port
	// * host
	localURLMap map[string]*url.URL
	// logger is the proxy logger.
	logger log.Logger
}

// NewHTTPProxy creates a new direct HTTPProxy, everything will be proxied to
// localURL.
func NewHTTPProxy(localURL *url.URL, logger log.Logger) *HTTPProxy {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	p := &HTTPProxy{
		localURL: localURL,
		logger:   logger,
	}
	p.ReverseProxy.Director = p.Director

	return p
}

// NewMultiHTTPProxy creates a new dispatching HTTPProxy, requests may go to
// different backends based on localURLMap.
func NewMultiHTTPProxy(localURLMap map[string]*url.URL, logger log.Logger) *HTTPProxy {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	p := &HTTPProxy{
		localURLMap: localURLMap,
		logger:      logger,
	}
	p.ReverseProxy.Director = p.Director

	return p
}

// Proxy is a ProxyFunc.
func (p *HTTPProxy) Proxy(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	switch msg.ForwardedProto {
	case proto.HTTP, proto.HTTPS:
		// ok
	default:
		p.logger.Log(
			"level", 0,
			"msg", "unsupported protocol",
			"ctrlMsg", msg,
		)
		return
	}

	rw, ok := w.(http.ResponseWriter)
	if !ok {
		p.logger.Log(
			"level", 0,
			"msg", "expected http.ResponseWriter",
			"ctrlMsg", msg,
		)
	}

	req, err := http.ReadRequest(bufio.NewReader(r))
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "failed to read request",
			"ctrlMsg", msg,
			"err", err,
		)
		return
	}

	setXForwardedFor(req.Header, msg.RemoteAddr)
	req.URL.Host = msg.ForwardedHost

	p.ServeHTTP(rw, req)
}

// Director is ReverseProxy Director it changes request URL so that the request
// is correctly routed based on localURL and localURLMap. If no URL can be found
// the request is canceled.
func (p *HTTPProxy) Director(req *http.Request) {
	orig := *req.URL

	target := p.localURLFor(req.URL)
	if target == nil {
		p.logger.Log(
			"level", 1,
			"msg", "no target",
			"url", req.URL,
		)

		_, cancel := context.WithCancel(req.Context())
		cancel()

		return
	}

	req.URL.Host = target.Host
	req.URL.Scheme = target.Scheme
	req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)

	targetQuery := target.RawQuery
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}

	req.Host = req.URL.Host

	p.logger.Log(
		"level", 2,
		"action", "url rewrite",
		"from", &orig,
		"to", req.URL,
	)
}

func singleJoiningSlash(a, b string) string {
	if a == "" || a == "/" {
		return b
	}
	if b == "" || b == "/" {
		return a
	}

	return path.Join(a, b)
}

func (p *HTTPProxy) localURLFor(u *url.URL) *url.URL {
	if len(p.localURLMap) == 0 {
		return p.localURL
	}

	// try host and port
	hostPort := u.Host
	if addr := p.localURLMap[hostPort]; addr != nil {
		return addr
	}

	// try port
	host, port, _ := net.SplitHostPort(hostPort)
	if addr := p.localURLMap[port]; addr != nil {
		return addr
	}

	// try host
	if addr := p.localURLMap[host]; addr != nil {
		return addr
	}

	return p.localURL
}
