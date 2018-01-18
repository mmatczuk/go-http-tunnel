package main

import (
	"os"
	"testing"
)

func TestCacheDir(t *testing.T) {
	os.Setenv("HOME", "/home/tunneld")
	dir := cacheDir("")
	expectedDefaultDir := "/home/tunneld/.cache/autocerts"
	if dir != expectedDefaultDir {
		t.Errorf("expected %s, got %s", expectedDefaultDir, dir)
	}
	dir = cacheDir("/custom/cache/folder/")
	expectedDefaultDir = "/custom/cache/folder/autocerts"
	if dir != expectedDefaultDir {
		t.Errorf("expected %s, got %s", expectedDefaultDir, dir)
	}
}

func TestTrimPort(t *testing.T) {
	bindIP := "192.18.1.1:9000"
	if trimPort(bindIP) != "192.18.1.1" {
		t.Error("trim should return the ip only")
	}
	bindHost := "localhost:3000"
	if trimPort(bindHost) != "localhost" {
		t.Error("trim should return the host only")
	}
	bindIPWithoutPort := "10.0.0.1"
	if trimPort(bindIPWithoutPort) != "10.0.0.1" {
		t.Error("trim should return the ip only")
	}
	bindHostWithoutPort := "localhost"
	if trimPort(bindHostWithoutPort) != "localhost" {
		t.Error("trim should return the host only")
	}
	if trimPort("") != "" {
		t.Error("trim should return an empty string")
	}
}
