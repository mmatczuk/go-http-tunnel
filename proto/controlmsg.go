package proto

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

const (
	HTTPProtocol = "http"
)

// Action represents type of ControlMsg request.
type Action int

// ControlMessage actions.
const (
	RequestClientSession Action = iota
)

// ControlMessage headers
const (
	ForwardedHeader = "Forwarded"
)

// ControlMessage is sent from server to client to establish tunneled connection.
type ControlMessage struct {
	Action       Action
	Protocol     string
	ForwardedFor string
	ForwardedBy  string
	URLPath      string
}

var xffRegexp = regexp.MustCompile("(for|by|proto|path)=([^;$]+)")

// NewControlMessage creates control message based on `Forwarded` http header.
func ParseControlMessage(h http.Header) (*ControlMessage, error) {
	v := h.Get(ForwardedHeader)
	if v == "" {
		return nil, errors.New("missing Forwarded header")
	}

	var msg = &ControlMessage{}

	for _, i := range xffRegexp.FindAllStringSubmatch(v, -1) {
		switch i[1] {
		case "for":
			msg.ForwardedFor = i[2]
		case "by":
			msg.ForwardedBy = i[2]
		case "proto":
			msg.Protocol = i[2]
		case "path":
			msg.URLPath = i[2]
		}
	}

	return msg, nil
}

// WriteTo writes ControlMessage to `Forwarded` http header, "by" and "for" parameters
// take form of full IP and port.
//
// If the server receiving proxied requests requires some address-based functionality,
// this parameter MAY instead contain an IP address (and, potentially, a port number)
//
// see https://tools.ietf.org/html/rfc7239.
func (c *ControlMessage) WriteTo(h http.Header) {
	h.Set(ForwardedHeader, fmt.Sprintf("for=%s; by=%s; proto=%s; path=%s",
		c.ForwardedFor, c.ForwardedBy, c.Protocol, c.URLPath))
}
