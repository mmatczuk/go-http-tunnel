// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/mmatczuk/go-http-tunnel/log"
)

func transfer(dst io.Writer, src io.Reader, logger log.Logger) {
	n, err := io.Copy(dst, src)
	if err != nil {
		if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "CANCEL") {
			logger.Log(
				"level", 2,
				"msg", "copy error",
				"err", err,
			)
		}
	}

	logger.Log(
		"level", 3,
		"action", "transferred",
		"bytes", n,
	)
}

func setXForwardedFor(h http.Header, remoteAddr string) {
	clientIP, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := h["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		h.Set("X-Forwarded-For", clientIP)
	}
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
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
