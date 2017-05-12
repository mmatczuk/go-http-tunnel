package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"sort"

	"gopkg.in/yaml.v2"

	"github.com/cenkalti/backoff"
	"github.com/google/gops/agent"
	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/cmd/cmd"
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

	if opts.debug {
		if err := agent.Listen(nil); err != nil {
			fatal("gops agent failed to start: %s", err)
		}
	}

	logger, err := cmd.NewLogger(opts.logTo, opts.logLevel)
	if err != nil {
		fatal("failed to init logger: %s", err)
	}

	// read configuration file
	config, err := loadConfiguration(opts.config)
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
		tunnels := make(map[string]*TunnelConfig)
		for _, arg := range opts.args {
			t, ok := config.Tunnels[arg]
			if !ok {
				fatal("no such tunnel %q", arg)
			}
			tunnels[arg] = t
		}
		config.Tunnels = tunnels
	}

	cert, err := tls.LoadX509KeyPair(config.TLSCrt, config.TLSKey)
	if err != nil {
		fatal("failed to load certificate: %s", err)
	}

	b, err := yaml.Marshal(config)
	if err != nil {
		fatal("failed to load config: %s", err)
	}
	logger.Log("config", string(b))

	client := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      config.ServerAddr,
		TLSClientConfig: tlsConfig(cert, config),
		Backoff:         expBackoff(config.Backoff),
		Tunnels:         tunnels(config.Tunnels),
		Proxy:           proxy(config.Tunnels, logger),
		Logger:          logger,
	})

	if err := client.Start(); err != nil {
		fatal("%s", err)
	}
}

func tlsConfig(cert tls.Certificate, config *Config) *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: config.InsecureSkipVerify,
	}
}

func expBackoff(config *BackoffConfig) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = config.InitialInterval
	b.Multiplier = config.Multiplier
	b.MaxInterval = config.MaxInterval
	b.MaxElapsedTime = config.MaxElapsedTime

	return b
}

func tunnels(m map[string]*TunnelConfig) map[string]*proto.Tunnel {
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

func proxy(m map[string]*TunnelConfig, logger log.Logger) tunnel.ProxyFunc {
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
