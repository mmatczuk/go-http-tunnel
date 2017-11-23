// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proto

import (
	"fmt"
	"net/http"
)

// Protocol HTTP headers.
const (
	HeaderError = "X-Error"

	HeaderAction        = "X-Action"
	HeaderProtocol      = "X-Proto"
	HeaderForwardedFor  = "X-Forwarded-For"
	HeaderForwardedHost = "X-Forwarded-Host"
)

// Known actions.
const (
	ActionProxy = "proxy"
)

// Known protocol types.
const (
	HTTP = "http"
	TCP  = "tcp"
	TCP4 = "tcp4"
	TCP6 = "tcp6"
	UNIX = "unix"
	WS   = "ws"
)

// ControlMessage is sent from server to client before streaming data. It's
// used to inform client about the data and action to take. Based on that client
// routes requests to backend services.
type ControlMessage struct {
	Action        string
	Protocol      string
	ForwardedFor  string
	ForwardedHost string
	Path          string
}

// ReadControlMessage reads ControlMessage from HTTP headers.
func ReadControlMessage(h http.Header) (*ControlMessage, error) {
	msg := ControlMessage{
		Action:        h.Get(HeaderAction),
		Protocol:      h.Get(HeaderProtocol),
		ForwardedFor:  h.Get(HeaderForwardedFor),
		ForwardedHost: h.Get(HeaderForwardedHost),
	}

	var missing []string

	if msg.Action == "" {
		missing = append(missing, HeaderAction)
	}
	if msg.Protocol == "" {
		missing = append(missing, HeaderProtocol)
	}
	if msg.ForwardedFor == "" {
		missing = append(missing, HeaderForwardedFor)
	}
	if msg.ForwardedHost == "" {
		missing = append(missing, HeaderForwardedHost)
	}

	if len(missing) != 0 {
		return nil, fmt.Errorf("missing headers: %s", missing)
	}

	return &msg, nil
}

// WriteToHeader writes ControlMessage to HTTP header.
func (c *ControlMessage) WriteToHeader(h http.Header) {
	h.Set(HeaderAction, string(c.Action))
	h.Set(HeaderProtocol, c.Protocol)
	h.Set(HeaderForwardedFor, c.ForwardedFor)
	h.Set(HeaderForwardedHost, c.ForwardedHost)
}
