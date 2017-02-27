package proto

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

// Action represents type of ControlMessage.
type Action int

// ControlMessage actions.
const (
	Proxy Action = iota
)

// ControlMessage headers
const (
	ErrorHeader     = "Error"
	ForwardedHeader = "Forwarded"
)

// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only), "udp",
// "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4" (IPv4-only),
// "ip6" (IPv6-only), "unix", "unixgram" and "unixpacket".
const (
	HTTP = "http"
	TCP  = "tcp"
	TCP4 = "tcp4"
	TCP6 = "tcp6"
	UNIX = "unix"
	WS   = "ws"
)

// ControlMessage is sent from server to client to establish tunneled
// connection.
type ControlMessage struct {
	Action       Action
	Protocol     string
	ForwardedFor string
	ForwardedBy  string
	Path         string
}

var xffRegexp = regexp.MustCompile("(proto|for|by|path)=([^;$]+)")

// ParseControlMessage creates new ControlMessage based on "Forwarded" http
// header.
func ParseControlMessage(h http.Header) (*ControlMessage, error) {
	v := h.Get(ForwardedHeader)
	if v == "" {
		return nil, errors.New("missing Forwarded header")
	}

	var msg = &ControlMessage{}

	for _, i := range xffRegexp.FindAllStringSubmatch(v, -1) {
		switch i[1] {
		case "proto":
			msg.Protocol = i[2]
		case "for":
			msg.ForwardedFor = i[2]
		case "by":
			msg.ForwardedBy = i[2]
		case "path":
			msg.Path = i[2]
		}
	}

	return msg, nil
}

// Update writes ControlMessage to "Forwarded" http header, "by" and "for"
// parameters take form of full IP and port.
//
// See Forwarded header specification https://tools.ietf.org/html/rfc7239.
func (c *ControlMessage) Update(h http.Header) {
	v := fmt.Sprintf("proto=%s; for=%s; by=%s; path=%s", c.Protocol, c.ForwardedFor, c.ForwardedBy, c.Path)
	h.Set(ForwardedHeader, v)
}
