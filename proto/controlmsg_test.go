package proto

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestControlMessage_WriteParse(t *testing.T) {
	msg := &ControlMessage{
		Protocol:     "tcp",
		ForwardedFor: "127.0.0.1:58104",
		ForwardedBy:  "127.0.0.1:7777",
		URLPath:      "/some/path",
	}

	h := make(http.Header)
	msg.WriteTo(h)
	actual, err := ParseControlMessage(h)

	assert.Nil(t, err)
	assert.Equal(t, msg, actual)
}
