// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/http2"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	tunnel "github.com/hons82/go-http-tunnel"
	"github.com/hons82/go-http-tunnel/connection"
	"github.com/hons82/go-http-tunnel/log"
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

	keepAlive, err := connection.Parse(opts.keepAlive)
	if err != nil {
		fatal("failed to parse KeepaliveConfig: %s", err)
	}

	debounceLog, err := time.ParseDuration(opts.debounceLog)
	if err != nil {
		fatal("failed to parse keepalive interval [%s], [%v]", opts.debounceLog, err)
	}
	debounced := &tunnel.Debounced{
		Execute: debounce.New(debounceLog),
	}

	// setup server
	server, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:          opts.tunnelAddr,
		SNIAddr:       opts.sniAddr,
		AutoSubscribe: autoSubscribe,
		TLSConfig:     tlsconf,
		Logger:        logger,
		KeepAlive:     *keepAlive,
		Debounce:      *debounced,
	})
	if err != nil {
		fatal("failed to create server: %s", err)
	}

	if !autoSubscribe {
		// First load immediatly
		server.LoadAllowedTunnels(opts.clients)

		// Watch for the file to change
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			fatal("NewWatcher failed: %s", err)
			logger.Log(
				"level", 1,
				"action", "could not create file watcher",
				"err", err,
			)
		} else {
			defer watcher.Close()

			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						logger.Log(
							"level", 3,
							"action", "watched file changed",
							"file", event.Name,
							"action", event.Op.String(),
						)
						if event.Op&fsnotify.Write == fsnotify.Write {
							server.Clear()
							server.LoadAllowedTunnels(event.Name)
						}
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						logger.Log(
							"level", 2,
							"action", "error watching file",
							"err", err,
						)
					}
				}
			}()

			err = watcher.Add(opts.clients)
			if err != nil {
				logger.Log(
					"level", 1,
					"action", "add watch failed",
					"file", opts.clients,
					"err", err,
				)
			}
		}
	}

	// start API
	if opts.apiAddr != "" {
		go func() {
			logger.Log(
				"level", 1,
				"action", "start api",
				"addr", opts.apiAddr,
			)
			go initAPIServer(&ApiConfig{
				Addr:   opts.apiAddr,
				Server: server,
				Logger: logger,
			})
		}()
	}

	// start HTTP
	if opts.httpAddr != "" {
		go func() {
			s := &http.Server{
				Addr: opts.httpAddr,
			}
			if opts.httpsAddr != "" {
				logger.Log(
					"level", 1,
					"action", "start http redirect",
					"addr", opts.httpAddr,
				)

				_, tlsPort, err := net.SplitHostPort(opts.httpsAddr)
				if err != nil {
					fatal("failed to get https port: %s", err)
				}
				s.Handler = http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						host, _, err := net.SplitHostPort(r.Host)
						if err != nil {
							host = r.Host
						}
						u := r.URL
						u.Host = net.JoinHostPort(host, tlsPort)
						u.Scheme = "https"
						http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
					},
				)
			} else {
				logger.Log(
					"level", 1,
					"action", "start http",
					"addr", opts.httpAddr,
				)
				s.Handler = server
			}
			fatal("failed to start HTTP: %s", s.ListenAndServe())
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
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
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
		Certificates:           []tls.Certificate{cert},
		ClientAuth:             clientAuth,
		ClientCAs:              roots,
		SessionTicketsDisabled: true,
		MinVersion:             tls.VersionTLS12,
		NextProtos:             []string{"h2"},
	}, nil
}

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
