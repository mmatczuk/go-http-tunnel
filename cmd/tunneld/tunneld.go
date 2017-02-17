package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/http2"

	"github.com/google/gops/agent"
	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/cmd/cmd"
	"github.com/mmatczuk/go-http-tunnel/id"
)

func main() {
	opts := parseArgs()

	if opts.version {
		fmt.Println(version)
		return
	}

	if opts.debug {
		if err := agent.Listen(nil); err != nil {
			fatal("gops agent failed to start: %s", err)
		}
	}

	logger, err := cmd.NewLogger(opts.logTo, opts.logLevel)
	if err != nil {
		fatal("failed to init logger: %s", err)
	}

	// load certs
	cert, err := tls.LoadX509KeyPair(opts.tlsCrt, opts.tlsKey)
	if err != nil {
		fatal("failed to load certificate: %s", err)
	}

	// setup server
	server, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:      opts.tunnelAddr,
		TLSConfig: tlsConfig(cert),
		Logger:    logger,
	})
	if err != nil {
		fatal("failed to create server: %s", err)
	}

	if opts.clients == "" {
		logger.Log(
			"level", 0,
			"msg", "No clients",
		)
	} else {
		for _, c := range strings.Split(opts.clients, ",") {
			if c == "" {
				fatal("empty client id")
			}
			identifier := id.ID{}
			err := identifier.UnmarshalText([]byte(c))
			if err != nil {
				fatal("invalid identifier %q: %s", c, err)
			}
			server.Subscribe(identifier)
		}
	}

	// start HTTP
	if opts.httpAddr != "" {
		go func() {
			logger.Log(
				"level", 1,
				"action", "start http",
				"addr", opts.httpAddr,
			)

			fatal("failed to start HTTP: %s", http.ListenAndServe(opts.httpAddr, server))
		}()
	}

	// start HTTPS
	if opts.httpsAddr != "" {
		go func() {
			logger.Log(
				"level", 1,
				"action", "start https",
				"addr", opts.httpsAddr,
			)

			s := &http.Server{
				Addr:    opts.httpsAddr,
				Handler: server,
			}
			http2.ConfigureServer(s, nil)

			fatal("failed to start HTTPS: %s", s.ListenAndServeTLS(opts.tlsCrt, opts.tlsKey))
		}()
	}

	server.Start()
}

func tlsConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientAuth:               tls.RequestClientCert,
		SessionTicketsDisabled:   true,
		MinVersion:               tls.VersionTLS12,
		CipherSuites:             []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}
}

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
