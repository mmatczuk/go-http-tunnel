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
				Action:        "action",
				Protocol:      "protocol",
				ForwardedFor:  "forwarded_for",
				ForwardedHost: "forwarded_host",
			},
			nil,
		},
		{
			&ControlMessage{
				Protocol:      "protocol",
				ForwardedFor:  "forwarded_for",
				ForwardedHost: "forwarded_host",
			},
			errors.New("missing headers: [T-Action]"),
		},
		{
			&ControlMessage{
				Action:        "action",
				ForwardedFor:  "forwarded_for",
				ForwardedHost: "forwarded_host",
			},
			errors.New("missing headers: [T-Proto]"),
		},
		{
			&ControlMessage{
				Action:        "action",
				Protocol:      "protocol",
				ForwardedHost: "forwarded_host",
			},
			errors.New("missing headers: [T-Forwarded-For]"),
		},
		{
			&ControlMessage{
				Action:       "action",
				Protocol:     "protocol",
				ForwardedFor: "forwarded_for",
			},
			errors.New("missing headers: [T-Forwarded-By]"),
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
