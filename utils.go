package h2tun

import (
	"fmt"
	"io"

	"github.com/koding/logging"
)

func url(client *AllowedClient, path string) string {
	return fmt.Sprint("https://", client.Host, path)
}

type closeWriter interface {
	CloseWrite() error
}
type closeReader interface {
	CloseRead() error
}

func transfer(side string, dst io.Writer, src io.ReadCloser, log logging.Logger) {
	log.Debug("proxing")

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

	log.Debug("done proxing %d bytes", n)
}
