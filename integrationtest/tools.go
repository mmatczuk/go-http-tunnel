package integrationtest

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
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

// RandBytes creates a randomy initialised byte slice of length n.
func RandBytes(n int) []byte {
	b := make([]byte, n)
	read, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	if read != n {
		panic("read did not fill whole slice")
	}
	return b
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
