package tunnel

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/mmatczuk/tunnel/id"
)

var (
	a = id.NewFromString("A")
	b = id.NewFromString("B")
)

func TestRegistry_IsSubscribed(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)

	if !r.IsSubscribed(a) {
		t.Fatal("Client should be subscribed")
	}
	if r.IsSubscribed(b) {
		t.Fatal("Client should not be subscribed")
	}
}

func TestRegistry_UnsubscribeOnce(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)

	if r.Unsubscribe(a) == nil {
		t.Fatal("Unsubscribe should return RegistryItem")
	}
	if r.Unsubscribe(a) != nil {
		t.Fatal("Unsubscribe should return nil")
	}
}

func TestRegistry_UnsubscribeReturnsHosts(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)
	r.AddHost("host0", nil, a)
	r.AddHost("host1", nil, a)

	i := r.Unsubscribe(a)
	if !reflect.DeepEqual(i.Hosts, []string{"host0", "host1"}) {
		t.Fatal("RegistryItem should contain hosts")
	}
}

func TestRegistry_UnsubscribeReturnsListeners(t *testing.T) {
	t.Parallel()

	l0 := &net.TCPListener{}
	l1 := &net.TCPListener{}

	r := newRegistry()
	r.Subscribe(a)
	r.AddListener(l0, a)
	r.AddListener(l1, a)

	i := r.Unsubscribe(a)
	if !reflect.DeepEqual(i.Listeners, []net.Listener{l0, l1}) {
		t.Fatal("RegistryItem should contain hosts")
	}
}

func TestRegistry_AddHostOnlyToSubscribed(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	if err := r.AddHost("host0", nil, a); err != errClientNotSubscribed {
		t.Fatal("Adding host should be possible to subscribned clients only")
	}
}

func TestRegistry_AddHostAuth(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)
	r.AddHost("host0", &Auth{User: "A", Password: "B"}, a)

	_, auth, _ := r.Subscriber("host0")
	if !reflect.DeepEqual(auth, &Auth{User: "A", Password: "B"}) {
		t.Fatal("Expected auth")
	}
}

func TestRegistry_AddHostTrimsPort(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)
	r.AddHost("host0", nil, a)
	r.AddHost("host1:80", nil, a)

	tests := []string{
		"host0",
		"host0:80",
		"host0:8080",
		"host1",
		"host1:80",
		"host1:8080",
	}

	for _, tt := range tests {
		identifier, auth, ok := r.Subscriber(tt)
		if !ok {
			t.Fatal("Subscriber not found")
		}
		if auth != nil {
			t.Fatal("Unexpeted auth")
		}
		if identifier != a {
			t.Fatal("Unexpeted identifier")
		}
	}
}

func TestRegistry_AddHostOnce(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)
	r.AddHost("host0", nil, a)

	tests := []string{
		"host0",
		"host0:80",
		"host0:8080",
	}

	for _, tt := range tests {
		if err := r.AddHost(tt, nil, a); !strings.Contains(err.Error(), "occupied") {
			t.Log(tt)
			t.Errorf("Adding host %q should fail", tt)
		}
	}

	r.Subscribe(b)

	for _, tt := range tests {
		if err := r.AddHost(tt, nil, b); !strings.Contains(err.Error(), "occupied") {
			t.Log(err)
			t.Errorf("Adding host %q should fail", tt)
		}
	}
}

func TestRegistry_DeleteHost(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)
	r.AddHost("host0", nil, a)

	r.DeleteHost("host0", a)

	if _, _, ok := r.Subscriber("host0"); ok {
		t.Fatal("Should delete host for a")
	}

	i := r.Unsubscribe(a)
	if len(i.Hosts) != 0 {
		t.Fatal("Host was not deleted from item")
	}
}

func TestRegistry_DeleteOnlyOwnedHost(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(a)
	r.AddHost("host0", nil, a)

	r.DeleteHost("host0", b)

	if _, _, ok := r.Subscriber("host0"); !ok {
		t.Fatal("Should not delete host for b")
	}
}

func TestRegistry_AddListenerOnlyToSubscribed(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	if err := r.AddListener(&net.TCPListener{}, a); err != errClientNotSubscribed {
		t.Fatal("Adding listener should be possible to subscribned clients only")
	}
}

func TestRegistry_AddListenerOnce(t *testing.T) {
	t.Parallel()

	l := &net.TCPListener{}

	r := newRegistry()
	r.Subscribe(a)
	r.AddListener(l, a)

	if err := r.AddListener(l, a); err == nil {
		t.Fatal("Adding listenr should fail")
	}
}

func TestRegistry_DeleteListener(t *testing.T) {
	t.Parallel()

	l := &net.TCPListener{}

	r := newRegistry()
	r.Subscribe(a)
	r.AddListener(l, a)

	r.DeleteListener(l, a)

	i := r.Unsubscribe(a)
	if len(i.Listeners) != 0 {
		t.Fatal("Host was not deleted from item")
	}
}
