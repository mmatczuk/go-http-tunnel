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
)

// ControlMessage is sent from server to client to establish tunneled
// connection.
type ControlMessage struct {
	Action       Action
	Protocol     string
	ForwardedFor string
	ForwardedBy  string
}

var xffRegexp = regexp.MustCompile("(for|proto|by)=([^;$]+)")

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
		case "for":
			msg.ForwardedFor = i[2]
		case "by":
			msg.ForwardedBy = i[2]
		case "proto":
			msg.Protocol = i[2]
		}
	}

	return msg, nil
}

// Update writes ControlMessage to "Forwarded" http header, "by" and "for"
// parameters take form of full IP and port.
//
// See Forwarded header specification https://tools.ietf.org/html/rfc7239.
func (c *ControlMessage) Update(h http.Header) {
	v := fmt.Sprintf("for=%s; proto=%s; by=%s", c.ForwardedFor, c.Protocol, c.ForwardedBy)
	h.Set(ForwardedHeader, v)
}
