package tunnel

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

// DefaultDialer is a dialer with all fields set to the default zero values.
var DefaultDialer = websocket.DefaultDialer

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
	rw, ok := w.(http.ResponseWriter)
	if !ok {
		panic(fmt.Sprintf("Expected http.ResponseWriter got %T", w))
	}
	br := bufio.NewReader(r)

	req, err := http.ReadRequest(br)
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "failed to read request",
			"err", err,
		)
		return
	}
	req.URL.Host = msg.ForwardedBy

	// TODO add target selection
	target := p.localURL
	if target == nil {
		p.logger.Log(
			"level", 1,
			"msg", "no target",
			"host", msg.ForwardedBy,
		)
		return
	}

	// TODO add custom dialer
	dialer := DefaultDialer

	ws, resp, err := dialer.Dial(target.String(), proxyHeaders(req))
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "dial failed",
			"err", err,
		)
		return
	}
	defer ws.Close()

	fmt.Println(resp.StatusCode)

	copyHeader(rw.Header(), resp.Header)
	// TODO rw.WriteHeader(resp.StatusCode)
	rw.WriteHeader(http.StatusOK)
	f, ok := w.(http.Flusher)
	if !ok {
		p.logger.Log(
			"level", 0,
			"msg", "cannot flush headers",
			"err", err,
		)
	}
	f.Flush()

	done := make(chan struct{})
	go func() {
		transfer(flushWriter{rw}, ws.UnderlyingConn(), log.NewContext(p.logger).With(
			"dst", msg.ForwardedBy,
			"src", target,
		))
		close(done)
	}()

	transfer(ws.UnderlyingConn(), ioutil.NopCloser(br), log.NewContext(p.logger).With(
		"dst", target,
		"src", msg.ForwardedBy,
	))

	<-done
}

func proxyHeaders(req *http.Request) http.Header {
	h := http.Header{}

	// Pass headers from the incoming request to the dialer to forward them to
	// the final destinations.
	requestHeader := http.Header{}
	if origin := req.Header.Get("Origin"); origin != "" {
		requestHeader.Add("Origin", origin)
	}
	for _, prot := range req.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
		requestHeader.Add("Sec-WebSocket-Protocol", prot)
	}
	for _, cookie := range req.Header[http.CanonicalHeaderKey("Cookie")] {
		requestHeader.Add("Cookie", cookie)
	}

	// Set X-Forwarded-For headers too, code below is a part of
	// httputil.ReverseProxy.
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		h.Set("X-Forwarded-For", clientIP)
	}

	// Set the originating protocol of the incoming HTTP request. The SSL
	// might be terminated on our site and because we doing proxy adding
	// this would be helpful for applications on the backend.
	h.Set("X-Forwarded-Proto", "http")
	if req.TLS != nil {
		h.Set("X-Forwarded-Proto", "https")
	}

	return h
}
