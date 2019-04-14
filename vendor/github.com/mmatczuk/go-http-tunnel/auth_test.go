// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"reflect"
	"testing"
)

func TestNewAuth(t *testing.T) {
	tests := []struct {
		actual   string
		expected *Auth
	}{
		{"", nil},
		{"user", &Auth{User: "user"}},
		{"user:password", &Auth{User: "user", Password: "password"}},
		{"user:pass:word", &Auth{User: "user", Password: "pass:word"}},
	}

	for _, tt := range tests {
		if !reflect.DeepEqual(NewAuth(tt.actual), tt.expected) {
			t.Errorf("Invalid auth for %s", tt.actual)
		}
	}
}
