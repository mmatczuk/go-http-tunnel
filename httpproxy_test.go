// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"net/http"
	"net/url"
	"testing"
)

func TestHTTPProxy_LocalURLFor(t *testing.T) {
	urlA, _ := url.Parse("http://127.0.0.1:8001")
	urlB, _ := url.Parse("http://127.0.0.1:8002")
	urlWildcard, _ := url.Parse("http://127.0.0.1:8003")
	urlCatchAll, _ := url.Parse("http://127.0.0.1:8004")

	proxy := NewMultiHTTPProxy(map[string]*url.URL{
		"example.com:80":  urlA,
		"8080":            urlB,
		"*.domain.com":    urlWildcard,
		"*":               urlCatchAll,
	}, nil)

	tests := []struct {
		rawURL   string
		expected *url.URL
	}{
		{"http://example.com:80/path", urlA},
		{"http://localhost:8080/api", urlB},
		{"http://foo.domain.com/test", urlWildcard},
		{"http://sub.foo.domain.com/test", urlWildcard},
		{"http://unmatched.org/index", urlCatchAll},
	}

	for _, tt := range tests {
		u, _ := url.Parse(tt.rawURL)
		target := proxy.localURLFor(u)
		if target != tt.expected {
			t.Errorf("url %s: expected %v, got %v", tt.rawURL, tt.expected, target)
		}
	}
}

func TestHTTPProxy_ForwardProxyDirector(t *testing.T) {
	proxy := NewMultiHTTPProxy(map[string]*url.URL{}, nil)
	proxy.ForwardProxy = true

	req, err := http.NewRequest(http.MethodGet, "http://remote-server.com/test/path?q=1", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	proxy.Director(req)

	if req.Context().Err() != nil {
		t.Fatalf("expected context not canceled in forward proxy mode, got: %v", req.Context().Err())
	}
	if req.URL.Scheme != "http" {
		t.Errorf("expected scheme http, got %s", req.URL.Scheme)
	}
	if req.URL.Host != "remote-server.com" {
		t.Errorf("expected host remote-server.com, got %s", req.URL.Host)
	}
	if req.Host != "remote-server.com" {
		t.Errorf("expected req.Host remote-server.com, got %s", req.Host)
	}
	if req.URL.Path != "/test/path" {
		t.Errorf("expected path /test/path, got %s", req.URL.Path)
	}
}
