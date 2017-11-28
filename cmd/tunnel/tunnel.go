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
	"net/url"
	"os"
	"sort"

	"gopkg.in/yaml.v2"

	"github.com/cenkalti/backoff"
	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

func main() {
	opts, err := parseArgs()
	if err != nil {
		fatal(err.Error())
	}

	if opts.version {
		fmt.Println(version)
		return
	}

	logger := log.NewFilterLogger(log.NewStdLogger(), opts.logLevel)

	// read configuration file
	config, err := loadClientConfigFromFile(opts.config)
	if err != nil {
		fatal("configuration error: %s", err)
	}

	switch opts.command {
	case "id":
		cert, err := tls.LoadX509KeyPair(config.TLSCrt, config.TLSKey)
		if err != nil {
			fatal("failed to load key pair: %s", err)
		}
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			fatal("failed to parse certificate: %s", err)
		}
		fmt.Println(id.New(x509Cert.Raw))

		return
	case "list":
		var names []string
		for n := range config.Tunnels {
			names = append(names, n)
		}

		sort.Strings(names)

		for _, n := range names {
			fmt.Println(n)
		}

		return
	case "start":
		tunnels := make(map[string]*Tunnel)
		for _, arg := range opts.args {
			t, ok := config.Tunnels[arg]
			if !ok {
				fatal("no such tunnel %q", arg)
			}
			tunnels[arg] = t
		}
		config.Tunnels = tunnels
	}

	if len(config.Tunnels) == 0 {
		fatal("no tunnels")
	}

	tlsconf, err := tlsConfig(config)
	if err != nil {
		fatal("failed to configure tls: %s", err)
	}

	b, err := yaml.Marshal(config)
	if err != nil {
		fatal("failed to dump config: %s", err)
	}
	logger.Log("config", string(b))

	client, err := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      config.ServerAddr,
		TLSClientConfig: tlsconf,
		Backoff:         expBackoff(config.Backoff),
		Tunnels:         tunnels(config.Tunnels),
		Proxy:           proxy(config.Tunnels, logger),
		Logger:          logger,
	})
	if err != nil {
		fatal("failed to create client: %s", err)
	}

	if err := client.Start(); err != nil {
		fatal("failed to start tunnels: %s", err)
	}
}

func tlsConfig(config *ClientConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(config.TLSCrt, config.TLSKey)
	if err != nil {
		return nil, err
	}

	var roots *x509.CertPool
	if config.RootCA != "" {
		roots = x509.NewCertPool()
		rootPEM, err := ioutil.ReadFile(config.RootCA)
		if err != nil {
			return nil, err
		}
		if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
			return nil, err
		}
	}

	host, _, err := net.SplitHostPort(config.ServerAddr)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		ServerName:         host,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: roots == nil,
		RootCAs:            roots,
	}, nil
}

func expBackoff(c BackoffConfig) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = c.Interval
	b.Multiplier = c.Multiplier
	b.MaxInterval = c.MaxInterval
	b.MaxElapsedTime = c.MaxTime

	return b
}

func tunnels(m map[string]*Tunnel) map[string]*proto.Tunnel {
	p := make(map[string]*proto.Tunnel)

	for name, t := range m {
		p[name] = &proto.Tunnel{
			Protocol: t.Protocol,
			Host:     t.Host,
			Auth:     t.Auth,
			Addr:     t.RemoteAddr,
		}
	}

	return p
}

func proxy(m map[string]*Tunnel, logger log.Logger) tunnel.ProxyFunc {
	httpURL := make(map[string]*url.URL)
	tcpAddr := make(map[string]string)

	for _, t := range m {
		switch t.Protocol {
		case proto.HTTP:
			u, err := url.Parse(t.Addr)
			if err != nil {
				fatal("invalid tunnel address: %s", err)
			}
			httpURL[t.Host] = u
		case proto.TCP, proto.TCP4, proto.TCP6:
			tcpAddr[t.RemoteAddr] = t.Addr
		}
	}

	return tunnel.Proxy(tunnel.ProxyFuncs{
		HTTP: tunnel.NewMultiHTTPProxy(httpURL, log.NewContext(logger).WithPrefix("proxy", "HTTP")).Proxy,
		TCP:  tunnel.NewMultiTCPProxy(tcpAddr, log.NewContext(logger).WithPrefix("proxy", "TCP")).Proxy,
	})
}

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
