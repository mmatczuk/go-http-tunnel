package h2tun_test

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrew-d/id"
	"github.com/koding/h2tun"
	"github.com/koding/h2tun/proto"
	"github.com/koding/logging"
	"github.com/stretchr/testify/assert"
)

func TestTCP(t *testing.T) {
	logging.DefaultLevel = logging.DEBUG
	logging.DefaultHandler.SetLevel(logging.DEBUG)

	cert, err := loadTestCert()
	assert.Nil(t, err)
	clientID := idFromTLSCert(cert)

	listener, err := net.Listen("tcp", ":7777")
	assert.Nil(t, err)
	defer listener.Close()

	server, err := h2tun.NewServer(&h2tun.ServerConfig{
		TLSConfig:      tlsConfig(cert),
		AllowedClients: []*h2tun.AllowedClient{{ID: clientID, Host: "foobar.com", Listeners: []net.Listener{listener}}},
	})
	assert.Nil(t, err)
	server.Start()
	defer server.Close()

	client := h2tun.NewClient(&h2tun.ClientConfig{
		ServerAddr:      server.Addr(),
		TLSClientConfig: tlsConfig(cert),
		Proxy:           echoProxyFunc,
	})
	go client.Connect()
	defer client.Close()

	time.Sleep(time.Second)

	conn, err := net.Dial("tcp", "localhost:7777")
	assert.Nil(t, err)

	const testPayload = "this is a test"

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for _, c := range testPayload {
			_, err := conn.Write([]byte{byte(c)})
			assert.Nil(t, err)
			time.Sleep(time.Millisecond)
		}
		conn.Close()
		wg.Done()
	}()
	go func() {
		b := bytes.NewBuffer([]byte{})
		io.Copy(b, conn)
		assert.Equal(t, testPayload, b.String())
		wg.Done()
	}()
	wg.Wait()
}

func echoProxyFunc(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	io.Copy(w, r)
}

func TestHTTP(t *testing.T) {
	logging.DefaultLevel = logging.DEBUG
	logging.DefaultHandler.SetLevel(logging.DEBUG)

	cert, err := loadTestCert()
	assert.Nil(t, err)
	clientID := idFromTLSCert(cert)

	server, err := h2tun.NewServer(&h2tun.ServerConfig{
		TLSConfig:      tlsConfig(cert),
		AllowedClients: []*h2tun.AllowedClient{{ID: clientID, Host: "foobar.com"}},
	})
	assert.Nil(t, err)
	server.Start()
	defer server.Close()

	client := h2tun.NewClient(&h2tun.ClientConfig{
		ServerAddr:      server.Addr(),
		TLSClientConfig: tlsConfig(cert),
		Proxy:           echoHTTPProxyFunc,
	})
	go client.Connect()
	defer client.Close()

	time.Sleep(time.Second)

	s := httptest.NewServer(server)
	defer s.Close()

	const testPayload = "this is a test"

	_, port, _ := net.SplitHostPort(s.Listener.Addr().String())
	r, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://foobar.com:%s/some/path", port), strings.NewReader(testPayload))
	assert.Nil(t, err)

	resp, err := http.DefaultClient.Do(r)
	assert.Nil(t, err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Equal(t, testPayload, string(body))
}

func echoHTTPProxyFunc(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	req, err := http.ReadRequest(bufio.NewReader(r))
	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "text/plain")

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    0,
		Request:       req,
		Header:        headers,
		ContentLength: int64(len(body)),
		Body:          ioutil.NopCloser(bytes.NewReader(body)),
	}
	resp.Write(w)
}

func loadTestCert() (tls.Certificate, error) {
	return tls.LoadX509KeyPair("./test-fixtures/selfsigned.crt", "./test-fixtures/selfsigned.key")
}

func tlsConfig(cert tls.Certificate) *tls.Config {
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

func idFromTLSCert(cert tls.Certificate) id.ID {
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		panic(err)
	}

	return idFromX509Cert(x509Cert)
}

func idFromX509Cert(cert *x509.Certificate) id.ID {
	return id.New(cert.Raw)
}
