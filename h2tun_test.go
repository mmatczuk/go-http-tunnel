package h2tun_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/andrew-d/id"
	"github.com/koding/h2tun"
	"github.com/koding/logging"
	"github.com/stretchr/testify/assert"
)

func TestHTTP(t *testing.T) {
	logging.DefaultLevel = logging.DEBUG

	cert, err := loadTestCert()
	assert.Nil(t, err)
	clientID := idFromTLSCert(cert)

	server, err := h2tun.NewServer(tlsConfig(cert), []*h2tun.AllowedClient{
		{ID: clientID, Host: "foobar.com"},
	})
	assert.NotNil(t, err)
	defer server.Close()

	client := h2tun.NewClient(server.Addr().String(), tlsConfig(cert))
	go client.Connect()

	select {}
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
	// Get the x509 cert for the given TLS certificate.
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		panic(err)
	}

	return idFromX509Cert(x509Cert)
}

func idFromX509Cert(cert *x509.Certificate) id.ID {
	return id.New(cert.Raw)
}
