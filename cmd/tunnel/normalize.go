// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func normalizeAddress(addr string) (string, error) {
	// normalize port to addr
	if _, err := strconv.Atoi(addr); err == nil {
		addr = ":" + addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return fmt.Sprintf("%s:%s", host, port), nil
}

func normalizeURL(rawurl string) (string, error) {
	// check scheme
	s := strings.SplitN(rawurl, "://", 2)
	if len(s) > 1 {
		switch s[0] {
		case "http", "https":
		default:
			return "", fmt.Errorf("unsupported url schema, choose 'http' or 'https'")
		}
	} else {
		rawurl = fmt.Sprint("http://", rawurl)
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}

	if u.Path != "" && !strings.HasSuffix(u.Path, "/") {
		return "", fmt.Errorf("url must end with '/'")
	}

	return rawurl, nil
}
