package proto

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestControlMessage_WriteParse(t *testing.T) {
	t.Parallel()

	data := []struct {
		msg *ControlMessage
		err error
	}{
		{
			&ControlMessage{
				Action:       "action",
				Protocol:     "protocol",
				ForwardedFor: "host-for",
				ForwardedBy:  "host-by",
			},
			nil,
		},
		{
			&ControlMessage{
				Protocol:     "protocol",
				ForwardedFor: "host-for",
				ForwardedBy:  "host-by",
			},
			errors.New("missing headers: [T-Action]"),
		},
		{
			&ControlMessage{
				Action:       "action",
				ForwardedFor: "host-for",
				ForwardedBy:  "host-by",
			},
			errors.New("missing headers: [T-Proto]"),
		},
		{
			&ControlMessage{
				Action:      "action",
				Protocol:    "protocol",
				ForwardedBy: "host-by",
			},
			errors.New("missing headers: [T-Forwarded-For]"),
		},
		{
			&ControlMessage{
				Action:       "action",
				Protocol:     "protocol",
				ForwardedFor: "host-for",
			},
			errors.New("missing headers: [T-Forwarded-By]"),
		},
	}

	for i, tt := range data {
		h := http.Header{}
		tt.msg.Update(h)
		actual, err := ReadControlMessage(h)
		if tt.err != nil {
			if err == nil {
				t.Error(i, "expected error")
			} else if tt.err.Error() != err.Error() {
				t.Error(i, tt.err, err)
			}
		} else {
			if !reflect.DeepEqual(tt.msg, actual) {
				t.Error(i, tt.msg, actual)
			}
		}
	}
}
