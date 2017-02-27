package tunnel

import (
	"io"
	"net/http"
	"strings"

	"github.com/mmatczuk/go-http-tunnel/log"
)

type closeWriter interface {
	CloseWrite() error
}

type closeReader interface {
	CloseRead() error
}

func transfer(dst io.Writer, src io.ReadCloser, logger log.Logger) {
	n, err := io.Copy(dst, src)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "copy error",
			"err", err,
		)
	}

	if d, ok := dst.(closeWriter); ok {
		d.CloseWrite()
	}

	if s, ok := src.(closeReader); ok {
		s.CloseRead()
	} else {
		src.Close()
	}

	logger.Log(
		"level", 3,
		"action", "transferred",
		"bytes", n,
	)
}

func isWebSocketConn(r *http.Request) bool {
	return r.Method == "GET" && headerContains(r.Header["Connection"], "upgrade") &&
		headerContains(r.Header["Upgrade"], "websocket")
}

func headerContains(header []string, value string) bool {
	for _, h := range header {
		for _, v := range strings.Split(h, ",") {
			if strings.EqualFold(strings.TrimSpace(v), value) {
				return true
			}
		}
	}

	return false
}

func copyHeader(dst, src http.Header) {
	for k, v := range src {
		vv := make([]string, len(v))
		copy(vv, v)
		dst[k] = vv
	}
}

type countWriter struct {
	w     io.Writer
	count int64
}

func (cw *countWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	cw.count += int64(n)
	return
}

type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}
