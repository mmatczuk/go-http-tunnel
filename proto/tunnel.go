// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package proto

// Tunnel describes a single tunnel between client and server. When connecting
// client sends tunnels to server. If client gets connected server proxies
// connections to given Host and RemoteAddr to the client.
type Tunnel struct {
	// Disabled tunnel disabled
	Disabled bool
	// Protocol specifies tunnel protocol, must be one of protocols known
	// by the server.
	Protocol string
	// Host specified HTTP request host, it's required for HTTP and WS
	// tunnels.
	Host string
	// Auth specifies HTTP basic auth credentials in form "user:password",
	// if set server would protect HTTP and WS tunnels with basic auth.
	Auth string
	// RemoteAddr specifies TCP address server would listen on, it's required
	// for TCP tunnels.
	RemoteAddr string `yaml:"remote_addr"`
	// LocalAddr client local addr
	LocalAddr string `yaml:"local_addr"`
}

// RegisteredTunnel describes a single registered tunnel between client and server. When connecting
// registered client sends tunnels to server. If client gets connected server proxies
// connections to given Host and LocalAddr to the client.
type RegisteredTunnel struct {
	// Disabled tunnel disabled
	Disabled bool
	// Auth specifies HTTP basic auth credentials in form "user:password",
	// if set server would protect HTTP and WS tunnels with basic auth.
	Auth string
	// LocalAddr client local addr
	LocalAddr string `yaml:"local_addr"`
}
