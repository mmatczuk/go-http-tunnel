package proto

import (
	"fmt"
	"net/http"
)

// Protocol HTTP headers.
const (
	HeaderAction       = "T-Action"
	HeaderError        = "T-Error"
	HeaderForwardedBy  = "T-Forwarded-By"
	HeaderForwardedFor = "T-Forwarded-For"
	HeaderPath         = "T-Path"
	HeaderProtocol     = "T-Proto"
)

// Known actions.
const (
	ActionProxy string = "proxy"
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
	Action       string
	Protocol     string
	ForwardedFor string
	ForwardedBy  string
	Path         string
}

// ReadControlMessage reads ControlMessage from HTTP headers.
func ReadControlMessage(h http.Header) (*ControlMessage, error) {
	msg := ControlMessage{
		Action:       h.Get(HeaderAction),
		Protocol:     h.Get(HeaderProtocol),
		ForwardedFor: h.Get(HeaderForwardedFor),
		ForwardedBy:  h.Get(HeaderForwardedBy),
		Path:         h.Get(HeaderPath),
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
	if msg.ForwardedBy == "" {
		missing = append(missing, HeaderForwardedBy)
	}

	if len(missing) != 0 {
		return nil, fmt.Errorf("missing headers: %s", missing)
	}

	return &msg, nil
}

// Update writes ControlMessage to HTTP header.
func (c *ControlMessage) Update(h http.Header) {
	h.Set(HeaderAction, string(c.Action))
	h.Set(HeaderProtocol, c.Protocol)
	h.Set(HeaderForwardedFor, c.ForwardedFor)
	h.Set(HeaderForwardedBy, c.ForwardedBy)
	h.Set(HeaderPath, c.Path)
}
