package h2tun

import (
	"fmt"
	"io"
	"net/http"

	"github.com/koding/logging"
)

func url(host string) string {
	return fmt.Sprint("https://", host)
}

type closeWriter interface {
	CloseWrite() error
}
type closeReader interface {
	CloseRead() error
}

func transfer(side string, dst io.Writer, src io.ReadCloser, log logging.Logger) {
	n, err := io.Copy(dst, src)
	if err != nil {
		log.Error("%s: copy error: %s", side, err)
	}

	if d, ok := dst.(closeWriter); ok {
		d.CloseWrite()
	}

	if s, ok := src.(closeReader); ok {
		s.CloseRead()
	} else {
		src.Close()
	}

	log.Debug("Coppied %d bytes from %s", n, side)
}

func copyHeader(dst, src http.Header) {
	for k, v := range src {
		vv := make([]string, len(v))
		copy(vv, v)
		dst[k] = vv
	}
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
