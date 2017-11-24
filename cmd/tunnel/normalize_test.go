// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"
)

func TestNormalizeAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr     string
		expected string
		error    string
	}{
		{
			addr:     "22",
			expected: "127.0.0.1:22",
		},
		{
			addr:     ":22",
			expected: "127.0.0.1:22",
		},
		{
			addr:     "0.0.0.0:22",
			expected: "0.0.0.0:22",
		},
		{
			addr:  "0.0.0.0",
			error: "missing port",
		},
		{
			addr:  "",
			error: "missing port",
		},
	}

	for i, tt := range tests {
		actual, err := normalizeAddress(tt.addr)
		if actual != tt.expected {
			t.Errorf("[%d] expected %q got %q err: %s", i, tt.expected, actual, err)
		}
		if tt.error != "" && err == nil {
			t.Errorf("[%d] expected error", i)
		}
		if err != nil && (tt.error == "" || !strings.Contains(err.Error(), tt.error)) {
			t.Errorf("[%d] expected error contains %q, got %q", i, tt.error, err)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rawurl   string
		expected string
		error    string
	}{
		{
			rawurl:   "localhost",
			expected: "http://localhost",
		},
		{
			rawurl:   "localhost:80",
			expected: "http://localhost:80",
		},
		{
			rawurl:   "localhost:80/path/",
			expected: "http://localhost:80/path/",
		},
		{
			rawurl: "localhost:80/path",
			error:  "/",
		},
		{
			rawurl:   "https://localhost",
			expected: "https://localhost",
		},
		{
			rawurl:   "https://localhost:443",
			expected: "https://localhost:443",
		},
		{
			rawurl:   "https://localhost:443/path/",
			expected: "https://localhost:443/path/",
		},
		{
			rawurl: "https://localhost:443/path",
			error:  "/",
		},
		{
			rawurl: "ftp://localhost",
			error:  "unsupported url schema",
		},
	}

	for i, tt := range tests {
		actual, err := normalizeURL(tt.rawurl)
		if actual != tt.expected {
			t.Errorf("[%d] expected %q got %q, err: %s", i, tt.expected, actual, err)
		}
		if tt.error != "" && err == nil {
			t.Errorf("[%d] expected error", i)
		}
		if err != nil && (tt.error == "" || !strings.Contains(err.Error(), tt.error)) {
			t.Errorf("[%d] expected error contains %q, got %q", i, tt.error, err)
		}
	}

}
