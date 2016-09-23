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
	log.Debug("proxing %s", side)

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

	log.Debug("done proxing %s %d bytes", side, n)
}

func copyHeader(dst, src http.Header) {
	for k, v := range src {
		vv := make([]string, len(v))
		copy(vv, v)
		dst[k] = vv
	}
}
