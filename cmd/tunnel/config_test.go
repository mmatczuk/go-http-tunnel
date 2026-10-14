// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientConfig_ProxyProtocol(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "tunnel-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	confYAML := `
server_addr: "127.0.0.1:5223"
tunnels:
  proxy_chain:
    proto: proxy
    auth: "user:pass"
  wildcard_http:
    proto: http
    host: "*"
`
	confPath := filepath.Join(tmpDir, "tunnel.yml")
	if err := ioutil.WriteFile(confPath, []byte(confYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadClientConfigFromFile(confPath)
	if err != nil {
		t.Fatalf("loadClientConfigFromFile failed: %v", err)
	}

	if len(cfg.Tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(cfg.Tunnels))
	}

	proxyTunnel := cfg.Tunnels["proxy_chain"]
	if proxyTunnel == nil || proxyTunnel.Host != "*" {
		t.Errorf("expected proxy_chain host to default to *, got %v", proxyTunnel)
	}

	tunMap := tunnels(cfg.Tunnels)
	if tunMap["proxy_chain"].Protocol != "http" || tunMap["proxy_chain"].Host != "*" {
		t.Errorf("expected proto.HTTP with host * in tunnels map, got %v", tunMap["proxy_chain"])
	}

	if tunMap["wildcard_http"].Protocol != "http" || tunMap["wildcard_http"].Host != "*" {
		t.Errorf("expected wildcard_http in tunnels map, got %v", tunMap["wildcard_http"])
	}
}
