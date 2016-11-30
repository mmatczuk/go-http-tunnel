package main

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/koding/logging"
	"github.com/koding/multiconfig"
	"github.com/koding/tunnel"
	"github.com/koding/tunnel/proto"
	"github.com/mmatczuk/tunnel/tunneltest"
	tunnelproto "github.com/mmatczuk/tunnel/proto"
)

type config struct {
	Identifier string `required:"true"`
	ServerAddr string `required:"true"`
	DataDir    string `required:"true"`
	Debug      bool
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

	p, err := inMemoryFileServer(config.DataDir)
	if err != nil {
		logging.Fatal("Could not create proxy function: %s", err)
	}

	c, err := tunnel.NewClient(&tunnel.ClientConfig{
		Identifier: config.Identifier,
		ServerAddr: config.ServerAddr,
		Dial: func(network, address string) (net.Conn, error) {
			return tls.Dial(network, address, &tls.Config{
				InsecureSkipVerify: true,
			})
		},
		Proxy: p,
	})
	if err != nil {
		logging.Fatal("Could not create client: %s", err)
	}

	defer c.Close()
	c.Start()
}

func inMemoryFileServer(dir string) (tunnel.ProxyFunc, error) {
	f, err := tunneltest.InMemoryFileServer(dir)
	if err != nil {
		return nil, err
	}

	return func(remote net.Conn, msg *proto.ControlMessage) {
		r, err := http.ReadRequest(bufio.NewReader(remote))
		if err != nil {
			logging.Error("Could not read request", err)
		}

		f(remote, remote, &tunnelproto.ControlMessage{
			URLPath: r.URL.Path,
		})
	}, nil
}
