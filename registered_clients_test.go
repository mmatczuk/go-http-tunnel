// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel_test

import (
	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/id"
)

type registeredClientsProvider struct {
	cfg *tunnel.RegisteredClientConfig
}

func (p registeredClientsProvider) Get(clientID id.ID) (client *tunnel.RegisteredClientConfig, err error) {
	return p.cfg, nil
}
