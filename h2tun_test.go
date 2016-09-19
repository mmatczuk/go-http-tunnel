package h2tun_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/andrew-d/id"
	"github.com/koding/h2tun"
	"github.com/koding/logging"
	"github.com/stretchr/testify/assert"
)

func TestHTTP(t *testing.T) {
	logging.DefaultLevel = logging.DEBUG
	logging.DefaultHandler.SetLevel(logging.DEBUG)

	cert, err := loadTestCert()
	assert.Nil(t, err)
	clientID := idFromTLSCert(cert)

	listener, err := net.Listen("tcp", ":7777")
	assert.Nil(t, err)
	defer listener.Close()

	server, err := h2tun.NewServer(
		tlsConfig(cert),
		[]*h2tun.AllowedClient{
			{
				ID:        clientID,
				Host:      "foobar.com",
				Listeners: []net.Listener{listener},
			},
		},
	)
	assert.Nil(t, err)
	server.Start()
	defer server.Close()

	client := h2tun.NewClient(server.Addr().String(), tlsConfig(cert))
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
