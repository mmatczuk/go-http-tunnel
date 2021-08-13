// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package connection

import (
	"fmt"
	"net"
	"time"
)

// Default keepAlive configuration.
const (
	DefaultKeepAliveInterval = 25 * time.Second
)

// KeepAliveConfig defines if and how the keepAlive package is sent.
type KeepAliveConfig struct {
	KeepAliveInterval time.Duration `yaml:"interval"`
}

func (k *KeepAliveConfig) Set(conn net.Conn) error {
	return keepAlive(conn, k.KeepAliveInterval)
}

func (k *KeepAliveConfig) String() string {
	return fmt.Sprintf("KeepAlive { interval: %v }", (*k).KeepAliveInterval)
}

func Parse(interval string) (*KeepAliveConfig, error) {
	_interval, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse keepalive interval [%s], [%v]", interval, err)
	}
	return &KeepAliveConfig{
		KeepAliveInterval: _interval,
	}, nil
}

func NewDefaultKeepAliveConfig() *KeepAliveConfig {
	return &KeepAliveConfig{
		KeepAliveInterval: DefaultKeepAliveInterval,
	}
}

func SetDefaultKeepAlive(conn net.Conn) error {
	return keepAlive(conn, DefaultKeepAliveInterval)
}

func keepAlive(conn net.Conn, interval time.Duration) error {
	c, ok := conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("bad connection type: %T", c)
	}

	if err := c.SetKeepAlive(interval > 0); err != nil {
		return err
	}

	if interval > 0 {
		if err := c.SetKeepAlivePeriod(interval); err != nil {
			return err
		}
	}

	return nil
}
