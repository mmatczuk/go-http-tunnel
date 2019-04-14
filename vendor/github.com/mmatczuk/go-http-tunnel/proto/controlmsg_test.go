// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package proto

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestControlMessageWriteRead(t *testing.T) {
	t.Parallel()

	data := []struct {
		msg *ControlMessage
		err error
	}{
		{
			&ControlMessage{
				Action:         "action",
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			nil,
		},
		{
			&ControlMessage{
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Action]"),
		},
		{
			&ControlMessage{
				Action:        "action",
				ForwardedHost: "forwarded_host",
			},
			errors.New("missing headers: [X-Forwarded-Proto]"),
		},
		{
			&ControlMessage{
				Action:         "action",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Forwarded-Host]"),
		},
	}

	for i, tt := range data {
		r := http.Request{}
		r.Header = http.Header{}
		tt.msg.WriteToHeader(r.Header)

		actual, err := ReadControlMessage(&r)
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
