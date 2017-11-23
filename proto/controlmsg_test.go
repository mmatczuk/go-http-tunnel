// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by a BSD-style
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
				ForwardedFor:   "forwarded_for",
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			nil,
		},
		{
			&ControlMessage{
				ForwardedFor:   "forwarded_for",
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Action]"),
		},
		{
			&ControlMessage{
				Action:        "action",
				ForwardedFor:  "forwarded_for",
				ForwardedHost: "forwarded_host",
			},
			errors.New("missing headers: [X-Forwarded-Proto]"),
		},
		{
			&ControlMessage{
				Action:         "action",
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Forwarded-For]"),
		},
		{
			&ControlMessage{
				Action:         "action",
				ForwardedFor:   "forwarded_for",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Forwarded-Host]"),
		},
	}

	for i, tt := range data {
		h := http.Header{}
		tt.msg.WriteToHeader(h)
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
