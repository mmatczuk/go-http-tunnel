package tunnel

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/proto"
	"gopkg.in/yaml.v2"
)

type RegisteredClientInfo struct {
	Hostname string
	Tunnels  map[string]*proto.RegisteredTunnel
}

type ErrClientNotRegistered struct {
	ClientID id.ID
}

func (e ErrClientNotRegistered) Error() string {
	return fmt.Sprintf("Client %q not registered", e.ClientID.String())
}

func IsNotRegistered(err error) (ok bool) {
	if err == nil {
		return
	}
	_, ok = err.(ErrClientNotRegistered)
	return
}

type RegisteredClientConfig struct {
	Disabled    bool
	ID          id.ID
	Name        string
	Description string
	Hostname    string
	Tunnels     map[string]*proto.Tunnel
	Connections int
}

type RegisteredClientsProvider interface {
	Get(clientID id.ID) (client *RegisteredClientConfig, err error)
}

type RegisteredClientsFileSystemProvider struct {
	StorageDir string
}

func (p *RegisteredClientsFileSystemProvider) Get(clientID id.ID) (client *RegisteredClientConfig, err error) {
	base := filepath.Join(p.StorageDir, clientID.String())
	pth := filepath.Join(base, "config.yaml")
	var f io.ReadCloser
	if f, err = os.Open(pth); err != nil {
		if os.IsNotExist(err) {
			return nil, &ErrClientNotRegistered{clientID}
		}
		return nil, err
	}
	defer f.Close()
	var data []byte
	if data, err = ioutil.ReadAll(f); err != nil {
		return nil, err
	}
	client = &RegisteredClientConfig{}
	if err = yaml.Unmarshal(data, client); err != nil {
		return nil, err
	}
	client.ID = clientID
	if client.Tunnels == nil {
		client.Tunnels = map[string]*proto.Tunnel{}
	}

	if _, err := os.Stat(filepath.Join(base, "disabled")); err == nil {
		client.Disabled = true
	}
	return
}
