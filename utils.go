package tunnel

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/mmatczuk/tunnel/log"
	"github.com/mmatczuk/tunnel/proto"
)

func clientRequest(host string, msg *proto.ControlMessage, r io.Reader) *http.Request {
	if msg.Action != proto.Proxy {
		panic("Invalid action")
	}
	req, err := http.NewRequest(http.MethodPut, clientURL(host), r)
	if err != nil {
		panic(fmt.Sprintf("Could not create request: %s", err))
	}
	msg.Update(req.Header)

	return req
}

func clientURL(host string) string {
	return fmt.Sprint("https://", host)
}

func trimPort(hostPort string) (host string) {
	host, _, _ = net.SplitHostPort(hostPort)
	if host == "" {
		host = hostPort
	}
	return
}

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
		"action", "transfered",
		"bytes", n,
	)
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
