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
	"path"
	"path/filepath"

	"github.com/koding/logging"
	"github.com/mmatczuk/tunnel"
	"github.com/mmatczuk/tunnel/proto"
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

// InMemoryFileServer scans directory dir, loads all files to memory and returns
// a http ProxyFunc that maps URL paths to relative filesystem paths i.e. file
// ./data/foo/bar.zip will be available under URL host:port/data/foo/bar.zip.
func InMemoryFileServer(dir string) (tunnel.ProxyFunc, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get directory absoute path %q: %s", dir, err)
	}
	prefix, _ := path.Split(dir)

	mux := make(map[string][]byte)

	visit := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if err != nil {
			return err
		}

		b, err := ResponseBytes(path)
		if err != nil {
			return err
		}
		mux[path[len(prefix)-1:]] = b

		return nil
	}

	if err := filepath.Walk(dir, visit); err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %s", dir, err)
	}

	return func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
		b, ok := mux[msg.URLPath]
		if !ok {
			logging.Warning("Resource not found for %v", msg)
			resp := &http.Response{
				Status:     "404 Not found",
				StatusCode: 404,
				Proto:      "HTTP/1.0",
				ProtoMajor: 1,
				ProtoMinor: 0,
				Header:     make(http.Header),
			}
			resp.Write(w)
		}
		w.Write(b)
	}, nil
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

// DebugLogging makes koding logger print debug messages.
func DebugLogging() {
	logging.DefaultLevel = logging.DEBUG
	logging.DefaultHandler.SetLevel(logging.DEBUG)
}
