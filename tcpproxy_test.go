// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"testing"
)

func TestTCPProxy_LocalAddrFor(t *testing.T) {
	proxy := NewMultiTCPProxy(map[string]string{
		"target.com:2222": "127.0.0.1:22",
		"3306":            "127.0.0.1:13306",
		"*.internal.net":  "127.0.0.1:9000",
		"*":               "127.0.0.1:8000",
	}, nil)

	tests := []struct {
		hostPort string
		expected string
	}{
		{"target.com:2222", "127.0.0.1:22"},
		{"db.local:3306", "127.0.0.1:13306"},
		{"service.internal.net:80", "127.0.0.1:9000"},
		{"other.host:443", "127.0.0.1:8000"},
	}

	for _, tt := range tests {
		got := proxy.localAddrFor(tt.hostPort)
		if got != tt.expected {
			t.Errorf("hostPort %s: expected %s, got %s", tt.hostPort, tt.expected, got)
		}
	}
}
