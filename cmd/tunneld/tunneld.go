// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"

	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/log"
)

var (
	hostnameRegex = regexp.MustCompile(`^[[:alnum:]_][[:alnum:]_-]{0,62}(\.[[:alnum:]_]{1}[[:alnum:]_-]{0,62})*[\._]?$`)
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
	if opts.httpAddr != "" && !opts.letsEncrypt {
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
	if opts.httpsAddr != "" && !opts.letsEncrypt {
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
	if opts.letsEncrypt {
		go startAutocert(opts, server, logger)
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

func startAutocert(opts *options, server *tunnel.Server, logger log.Logger) {
	cacheDir, err := cacheDir(opts.letsEncryptCacheDir)
	if err != nil {
		fatal("failed to set cache directory for certs ", err)
	}
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy(server),
		Cache:      autocert.DirCache(cacheDir),
	}
	if opts.httpAddr == ":80" {
		opts.httpAddr = ""
	}
	if opts.httpsAddr == ":443" {
		opts.httpsAddr = ""
	}
	// Allow binding to a specific host
	if !validAddr(opts.httpAddr) || !validAddr(opts.httpsAddr) {
		fatal("invalid  http(s) address, port not allowed with Let's Encrypt enabled")
	}
	httpAddr := net.JoinHostPort(opts.httpAddr, "80")
	httpsAddr := net.JoinHostPort(opts.httpsAddr, "443")
	s := &http.Server{
		Addr:    httpsAddr,
		Handler: server,
	}
	s.TLSConfig = &tls.Config{
		GetCertificate: m.GetCertificate,
	}
	http2.ConfigureServer(s, nil)
	logger.Log(
		"level", 1,
		"action", "start https with Let's Encrypt support",
		"addr", "80,443",
	)
	go func() {
		fatal("failed to start HTTP: %s", http.ListenAndServe(httpAddr, m.HTTPHandler(http.HandlerFunc(redirect))))
	}()
	go func() {
		fatal("failed to start HTTPS: %s", s.ListenAndServeTLS("", ""))
	}()
}

func hostPolicy(server *tunnel.Server) autocert.HostPolicy {
	return func(_ context.Context, host string) error {
		_, _, subscribed := server.Subscriber(host)
		if !subscribed {
			return fmt.Errorf("acme/autocert: host `%s` not subscribed", host)
		}
		return nil
	}
}

// validAddr checks that addrs doesn't contains a port already
func validAddr(addr string) bool {
	if ip := net.ParseIP(addr); ip != nil {
		return true
	}
	if hostnameRegex.MatchString(addr) {
		return true
	}
	if addr == "" {
		return true
	}
	return false
}

func cacheDir(letsEncryptCacheDir string) (string, error) {
	if letsEncryptCacheDir != "" {
		return letsEncryptCacheDir, nil
	}
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(user.HomeDir, ".tunnel", "certs"), nil
}

func redirect(w http.ResponseWriter, r *http.Request) {
	r.URL.Scheme = "https"
	r.URL.Host = r.Host
	http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
}
