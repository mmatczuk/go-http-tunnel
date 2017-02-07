package tunnel

import (
	"net"
	"testing"

	"github.com/mmatczuk/tunnel/id"
)

func TestRegistry_Subscribe(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(id.NewFromString("A"))

	if ok := r.IsSubscribed(id.NewFromString("A")); !ok {
		t.Fatal("Client should be subscribed")
	}

	if i := r.Unsubscribe(id.NewFromString("A")); i == nil {
		t.Fatal("Unsubscribe should return RegistryItem")
	}
	if i := r.Unsubscribe(id.NewFromString("A")); i != nil {
		t.Fatal("Unsubscribe for not existing client should return null")
	}
}

func TestRegistry_AddHost(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	if err := r.AddHost("foobar", id.NewFromString("A")); err != errClientNotSubscribed {
		t.Fatal("AddHost to not subscribed client should fail")
	}

	r.Subscribe(id.NewFromString("A"))

	if err := r.AddHost("foobar:8080", id.NewFromString("A")); err != nil {
		t.Fatal("AddHost should succeed")
	}

	r.Subscribe(id.NewFromString("B"))

	if err := r.AddHost("foobar", id.NewFromString("B")); err == nil {
		t.Fatal("AddHost for duplicate host should fail")
	}

	if identifier, ok := r.Subscriber("foobar"); !ok || identifier != id.NewFromString("A") {
		t.Fatal("Wrong subscriber")
	}
	if identifier, ok := r.Subscriber("foobar:8080"); !ok || identifier != id.NewFromString("A") {
		t.Fatal("Wrong subscriber")
	}

	r.Unsubscribe(id.NewFromString("A"))

	if err := r.AddHost("foobar", id.NewFromString("B")); err != nil {
		t.Fatal("Unsubsribe failed to remove host")
	}
}

func TestRegistry_DeleteHost(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.Subscribe(id.NewFromString("A"))

	if err := r.AddHost("foobar:8080", id.NewFromString("A")); err != nil {
		t.Fatal("AddHost should succeed")
	}

	if identifier, ok := r.Subscriber("foobar"); !ok || identifier != id.NewFromString("A") {
		t.Fatal("Wrong subscriber")
	}

	if err := r.AddHost("foobar:8080", id.NewFromString("A")); err == nil {
		t.Fatal("AddHost for duplicate host should fail")
	}

	r.DeleteHost("foobar", id.NewFromString("A"))

	if _, ok := r.Subscriber("foobar"); ok {
		t.Fatal("DeleteHost failed to delete host")
	}

	if err := r.AddHost("foobar:8080", id.NewFromString("A")); err != nil {
		t.Fatal("AddHost should succeed")
	}

	r.Subscribe(id.NewFromString("B"))
	r.DeleteHost("foobar", id.NewFromString("B"))

	if _, ok := r.Subscriber("foobar"); !ok {
		t.Fatal("DeleteHost forgein host should have no effect")
	}
}

func TestRegistry_AddListener(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	if err := r.AddListener(&net.TCPListener{}, id.NewFromString("A")); err != errClientNotSubscribed {
		t.Fatal("AddListener to not subscribed client should fail")
	}

	r.Subscribe(id.NewFromString("A"))

	if err := r.AddListener(&net.TCPListener{}, id.NewFromString("A")); err != nil {
		t.Fatal("AddListener should succeed")
	}
}
