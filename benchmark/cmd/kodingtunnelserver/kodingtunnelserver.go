package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/koding/logging"
	"github.com/koding/multiconfig"
	"github.com/koding/tunnel"
	"github.com/mmatczuk/tunnel/tunneltest"
	"golang.org/x/net/http2"
)

type config struct {
	Addr        string `required:"true"`
	HTTPS       string `required:"true"`
	Host        string `required:"true"`
	TLSCertFile string `required:"true"`
	TLSKeyFile  string `required:"true"`
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

	s, err := tunnel.NewServer(&tunnel.ServerConfig{})
	if err != nil {
		logging.Fatal("Server creation failed: %s", err)
	}

	hostPort := config.Host
	_, port, _ := net.SplitHostPort(config.HTTPS)
	if port != "" {
		hostPort = fmt.Sprint(config.Host, ":", port)
	}
	s.AddHost(hostPort, config.Host)

	go func() {
		// Needed for Hijacking to work, Hijack and HTTP/2 are fundamentally incompatible
		srv := &http.Server{
			Addr:    config.Addr,
			Handler: s,
			TLSConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			},
		}
		logging.Fatal("HTTP: %s", srv.ListenAndServeTLS("", ""))
	}()

	h2srv := &http.Server{
		Addr:      config.HTTPS,
		Handler:   s,
		TLSConfig: tunneltest.TLSConfig(cert),
	}
	http2.ConfigureServer(h2srv, &http2.Server{})
	logging.Fatal("HTTP2: %s", h2srv.ListenAndServeTLS("", ""))
}
