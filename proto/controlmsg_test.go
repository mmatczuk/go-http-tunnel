package proto

import (
	"net/http"
	"reflect"
	"testing"
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
	if err != nil {
		t.Errorf("Parse error %s", err)
	}
	if !reflect.DeepEqual(msg, actual) {
		t.Errorf("Received %+v expected %+v", msg, actual)
	}
}
