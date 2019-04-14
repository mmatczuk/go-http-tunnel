// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package proto

import (
	"fmt"
	"net/http"
)

// Protocol HTTP headers.
const (
	HeaderError = "X-Error"

	HeaderAction         = "X-Action"
	HeaderForwardedHost  = "X-Forwarded-Host"
	HeaderForwardedProto = "X-Forwarded-Proto"
)

// Known actions.
const (
	ActionProxy = "proxy"
)

// Known protocol types.
const (
	HTTP  = "http"
	HTTPS = "https"

	TCP  = "tcp"
	TCP4 = "tcp4"
	TCP6 = "tcp6"
	UNIX = "unix"
)

// ControlMessage is sent from server to client before streaming data. It's
// used to inform client about the data and action to take. Based on that client
// routes requests to backend services.
type ControlMessage struct {
	Action         string
	ForwardedHost  string
	ForwardedProto string
	RemoteAddr     string
}

// ReadControlMessage reads ControlMessage from HTTP headers.
func ReadControlMessage(r *http.Request) (*ControlMessage, error) {
	msg := ControlMessage{
		Action:         r.Header.Get(HeaderAction),
		ForwardedHost:  r.Header.Get(HeaderForwardedHost),
		ForwardedProto: r.Header.Get(HeaderForwardedProto),
		RemoteAddr:     r.RemoteAddr,
	}

	var missing []string

	if msg.Action == "" {
		missing = append(missing, HeaderAction)
	}
	if msg.ForwardedHost == "" {
		missing = append(missing, HeaderForwardedHost)
	}
	if msg.ForwardedProto == "" {
		missing = append(missing, HeaderForwardedProto)
	}

	if len(missing) != 0 {
		return nil, fmt.Errorf("missing headers: %s", missing)
	}

	return &msg, nil
}

// WriteToHeader writes ControlMessage to HTTP header.
func (c *ControlMessage) WriteToHeader(h http.Header) {
	h.Set(HeaderAction, string(c.Action))
	h.Set(HeaderForwardedHost, c.ForwardedHost)
	h.Set(HeaderForwardedProto, c.ForwardedProto)
}
