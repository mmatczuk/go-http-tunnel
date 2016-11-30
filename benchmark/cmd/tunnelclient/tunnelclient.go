package main

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/andrew-d/id"
	"github.com/koding/logging"
	"github.com/koding/multiconfig"
	"github.com/mmatczuk/tunnel"
	"github.com/mmatczuk/tunnel/tunneltest"
)

type config struct {
	ServerAddr  string `required:"true"`
	TLSCertFile string `required:"true"`
	TLSKeyFile  string `required:"true"`
	DataDir     string `required:"true"`
	Debug       bool
}

func main() {
	m := multiconfig.New()
	config := new(config)
	m.MustLoad(config)
	m.MustValidate(config)
	logging.Info("Loaded config: %v", config)

	if config.Debug {
		tunneltest.DebugLogging()
	}

	cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		logging.Fatal("Failed to load TLS key pair: %s", err)
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		logging.Fatal("Failed to get x509 certificate from TLS: %s", err)
	}
	certID := id.New(x509Cert.Raw)
	b, _ := certID.MarshalText()
	logging.Info("Client using cert %q", string(b))

	p, err := tunneltest.InMemoryFileServer(config.DataDir)
	if err != nil {
		logging.Fatal("Could not create proxy function: %s", err)
	}

	c := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      config.ServerAddr,
		TLSClientConfig: tunneltest.TLSConfig(cert),
		Proxy:           p,
	})
	if err := c.Start(); err != nil {
		logging.Fatal("Client start failed: %s", err)
	}
	defer c.Stop()

	select {}
}
