package proto

import (
	"net/http"
	"reflect"
	"testing"
)

func TestControlMessage_WriteParse(t *testing.T) {
	t.Parallel()

	msg := &ControlMessage{
		Protocol:     "tcp",
		ForwardedFor: "127.0.0.1:58104",
		ForwardedBy:  "127.0.0.1:7777",
	}

	var h = http.Header{}
	msg.Update(h)
	actual, err := ParseControlMessage(h)
	if err != nil {
		t.Errorf("Parse error %s", err)
	}
	if !reflect.DeepEqual(msg, actual) {
		t.Errorf("Received %+v expected %+v", msg, actual)
	}
}
