package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheDir(t *testing.T) {
	dir, err := cacheDir("")
	if err != nil {
		t.Fatalf("Couln't get a cache directory %q", err)
	}
	endDir := filepath.Join(".tunnel", "/certs")
	if !strings.HasSuffix(dir, endDir) {
		t.Errorf("expected %s, got %s", "$HOME/.tunnel/certs", endDir)
	}
	dir, _ = cacheDir("/custom/cache/folder")
	expectedDefaultDir := "/custom/cache/folder"
	if dir != expectedDefaultDir {
		t.Errorf("expected %s, got %s", expectedDefaultDir, dir)
	}
}

func TestValidAddr(t *testing.T) {
	tests := []struct {
		addr  string
		valid bool
	}{
		{"", true},
		{"192.18.1.1", true},
		{"10.0.0.1:3423", false},
		{"::1", true},
		{"[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:80", false},
		{"localhost", true},
		{"localhost:9090", false},
		{"sub-domain.domain.com", true},
		{"sub-domain.domain.com:http", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if tt.valid && !validAddr(tt.addr) {
				t.Errorf("should be a valid address")
			}
			if !tt.valid && validAddr(tt.addr) {
				t.Error("should be a invalid address")
			}
		})
	}
}

func TestRedirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://my.domain.com/path/to/resource", nil)
	w := httptest.NewRecorder()
	redirect(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Errorf("invalid status code expected %d got %d ", http.StatusMovedPermanently, w.Code)
	}
	expectedURL := "https://my.domain.com/path/to/resource"
	resultURL := w.Header().Get("Location")
	if resultURL != expectedURL {
		t.Errorf("invalid url redirect expected %s got %s ", expectedURL, resultURL)
	}
}
