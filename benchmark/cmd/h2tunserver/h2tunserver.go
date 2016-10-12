package main

import (
	"crypto/tls"
	"net/http"

	"github.com/andrew-d/id"
	"github.com/koding/logging"
	"github.com/koding/multiconfig"
	"github.com/mmatczuk/h2tun"
	"github.com/mmatczuk/h2tun/h2tuntest"
	"golang.org/x/net/http2"
)

type config struct {
	Addr        string `required:"true"`
	HTTPS       string `required:"true"`
	Host        string `required:"true"`
	ClientID    string `required:"true"`
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
		h2tuntest.DebugLogging()
	}

	cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		logging.Fatal("Failed to load TLS key pair: %s", err)
	}

	clientID := new(id.ID)
	if err := clientID.UnmarshalText([]byte(config.ClientID)); err != nil {
		logging.Fatal("Failed to parse client cert ID: %s", err)
	}

	s, err := h2tun.NewServer(&h2tun.ServerConfig{
		Addr:           config.Addr,
		TLSConfig:      h2tuntest.TLSConfig(cert),
		AllowedClients: []*h2tun.AllowedClient{{ID: *clientID, Host: config.Host}},
	})
	if err != nil {
		logging.Fatal("Server creation failed: %s", err)
	}
	s.Start()

	h2srv := &http.Server{
		Addr:      config.HTTPS,
		Handler:   s,
		TLSConfig: h2tuntest.TLSConfig(cert),
	}
	http2.ConfigureServer(h2srv, &http2.Server{})

	logging.Fatal("HTTP2: %s", h2srv.ListenAndServeTLS("", ""))
}
