// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"testing"

	"github.com/mmatczuk/go-http-tunnel/id"
)

func TestRegistry_WildcardAndCatchAll(t *testing.T) {
	reg := newRegistry(nil)

	id1 := id.ID{1}
	id2 := id.ID{2}
	id3 := id.ID{3}

	reg.Subscribe(id1)
	reg.Subscribe(id2)
	reg.Subscribe(id3)

	item1 := &RegistryItem{
		Hosts: []*HostAuth{
			{Host: "specific.example.com", Auth: NewAuth("user1:pass1")},
		},
	}
	if err := reg.set(item1, id1); err != nil {
		t.Fatalf("set item1 failed: %v", err)
	}

	item2 := &RegistryItem{
		Hosts: []*HostAuth{
			{Host: "*.example.com", Auth: NewAuth("user2:pass2")},
		},
	}
	if err := reg.set(item2, id2); err != nil {
		t.Fatalf("set item2 failed: %v", err)
	}

	item3 := &RegistryItem{
		Hosts: []*HostAuth{
			{Host: "*", Auth: NewAuth("user3:pass3")},
		},
	}
	if err := reg.set(item3, id3); err != nil {
		t.Fatalf("set item3 failed: %v", err)
	}

	tests := []struct {
		host           string
		expectedID     id.ID
		expectedUser   string
		expectedPass   string
		expectedResult bool
	}{
		{
			host:           "specific.example.com:80",
			expectedID:     id1,
			expectedUser:   "user1",
			expectedPass:   "pass1",
			expectedResult: true,
		},
		{
			host:           "api.example.com:8080",
			expectedID:     id2,
			expectedUser:   "user2",
			expectedPass:   "pass2",
			expectedResult: true,
		},
		{
			host:           "sub.api.example.com",
			expectedID:     id2,
			expectedUser:   "user2",
			expectedPass:   "pass2",
			expectedResult: true,
		},
		{
			host:           "otherdomain.org",
			expectedID:     id3,
			expectedUser:   "user3",
			expectedPass:   "pass3",
			expectedResult: true,
		},
		{
			host:           "random.target.net:443",
			expectedID:     id3,
			expectedUser:   "user3",
			expectedPass:   "pass3",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		gotID, gotAuth, ok := reg.Subscriber(tt.host)
		if ok != tt.expectedResult {
			t.Errorf("host %s: expected ok=%v, got %v", tt.host, tt.expectedResult, ok)
			continue
		}
		if gotID != tt.expectedID {
			t.Errorf("host %s: expected ID %v, got %v", tt.host, tt.expectedID, gotID)
		}
		if gotAuth == nil || gotAuth.User != tt.expectedUser || gotAuth.Password != tt.expectedPass {
			t.Errorf("host %s: expected auth %s:%s, got %v", tt.host, tt.expectedUser, tt.expectedPass, gotAuth)
		}
	}

	// Verify unregister cleans up wildcard hosts properly
	reg.Unsubscribe(id2)
	gotID, _, ok := reg.Subscriber("api.example.com")
	if !ok || gotID != id3 {
		t.Errorf("after removing id2, api.example.com should fall back to catch-all id3, got ID %v, ok=%v", gotID, ok)
	}

	reg.Unsubscribe(id3)
	_, _, ok = reg.Subscriber("random.target.net")
	if ok {
		t.Errorf("after removing catch-all id3, random.target.net should not match")
	}
}
