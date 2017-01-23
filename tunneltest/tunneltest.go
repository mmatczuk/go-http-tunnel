// Package tunneltest contains common testing tools shared by unit tests,
// benchmarks and third party tests.
package tunneltest

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// EchoHTTP starts serving HTTP requests on listener l, it accepts connections,
// reads request body and writes is back in response.
func EchoHTTP(l net.Listener) {
	http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Body != nil {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				panic(err)
			}
			w.Write(body)
		}
	}))
}

// EchoTCP accepts connections and copies back received bytes.
func EchoTCP(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			io.Copy(conn, conn)
		}()
	}
}

// ResponseBytes returns http response containing file as body.
func ResponseBytes(file string) ([]byte, error) {
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header:     make(http.Header),
	}

	ctype := mime.TypeByExtension(filepath.Ext(file))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	resp.Header.Set("Content-Type", ctype)

	r, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %s", file, err)
	}
	defer r.Close()
	resp.Body = r

	b := new(bytes.Buffer)
	if err := resp.Write(b); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// TLSConfig returns valid http/2 tls configuration that can be used by both
// client and server.
func TLSConfig(cert tls.Certificate) *tls.Config {
	c := &tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientAuth:               tls.RequestClientCert,
		SessionTicketsDisabled:   true,
		InsecureSkipVerify:       true,
		MinVersion:               tls.VersionTLS12,
		CipherSuites:             []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}
	c.BuildNameToCertificate()
	return c
}
