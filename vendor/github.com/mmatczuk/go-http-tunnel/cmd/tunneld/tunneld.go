// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/http2"

	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/log"
)

func main() {
	opts := parseArgs()

	if opts.version {
		fmt.Println(version)
		return
	}

	fmt.Println(banner)

	logger := log.NewFilterLogger(log.NewStdLogger(), opts.logLevel)

	tlsconf, err := tlsConfig(opts)
	if err != nil {
		fatal("failed to configure tls: %s", err)
	}

	autoSubscribe := opts.clients == ""

	// setup server
	server, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:          opts.tunnelAddr,
		AutoSubscribe: autoSubscribe,
		TLSConfig:     tlsconf,
		Logger:        logger,
	})
	if err != nil {
		fatal("failed to create server: %s", err)
	}

	if !autoSubscribe {
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

func tlsConfig(opts *options) (*tls.Config, error) {
	// load certs
	cert, err := tls.LoadX509KeyPair(opts.tlsCrt, opts.tlsKey)
	if err != nil {
		return nil, err
	}

	// load root CA for client authentication
	clientAuth := tls.RequireAnyClientCert
	var roots *x509.CertPool
	if opts.rootCA != "" {
		roots = x509.NewCertPool()
		rootPEM, err := ioutil.ReadFile(opts.rootCA)
		if err != nil {
			return nil, err
		}
		if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
			return nil, err
		}
		clientAuth = tls.RequireAndVerifyClientCert
	}

	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientAuth:               clientAuth,
		ClientCAs:                roots,
		SessionTicketsDisabled:   true,
		MinVersion:               tls.VersionTLS12,
		CipherSuites:             []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}, nil
}

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
