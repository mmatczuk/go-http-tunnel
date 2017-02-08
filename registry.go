package tunnel

import (
	"fmt"
	"net"
	"sync"

	"github.com/mmatczuk/tunnel/id"
)

// RegistryItem holds information about hosts and listeners associated with a
// client.
type RegistryItem struct {
	Hosts     []string
	Listeners []net.Listener
}

type hostInfo struct {
	identifier id.ID
	auth       *Auth
}

// registry manages client tunnels information.
type registry struct {
	items map[id.ID]*RegistryItem
	hosts map[string]*hostInfo
	mu    sync.RWMutex
}

// newRegistry creates new registry.
func newRegistry() *registry {
	return &registry{
		items: make(map[id.ID]*RegistryItem),
		hosts: make(map[string]*hostInfo, 0),
	}
}

// Subscribe adds new client to registry, this method is idempotent.
func (r *registry) Subscribe(identifier id.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[identifier]; ok {
		return
	}

	r.items[identifier] = &RegistryItem{
		Hosts:     make([]string, 0),
		Listeners: make([]net.Listener, 0),
	}
}

// IsSubscribed returns true if client is subscribed to registry.
func (r *registry) IsSubscribed(identifier id.ID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.items[identifier]
	return ok
}

// Subscriber returns client identifier assigned to given host.
func (r *registry) Subscriber(hostPort string) (id.ID, *Auth, bool) {
	host := trimPort(hostPort)

	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.hosts[host]
	if !ok {
		return id.ID{}, nil, false
	}

	return h.identifier, h.auth, ok
}

// Unsubscribe removes client from registy and returns it's RegistryItem.
func (r *registry) Unsubscribe(identifier id.ID) *RegistryItem {
	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[identifier]
	if !ok {
		return nil
	}

	for _, h := range i.Hosts {
		delete(r.hosts, h)
	}

	delete(r.items, identifier)

	return i
}

// AddHost assigns host to client unless the host is not already taken.
func (r *registry) AddHost(hostPort string, auth *Auth, identifier id.ID) error {
	host := trimPort(hostPort)

	r.mu.Lock()
	defer r.mu.Unlock()

	if auth != nil && auth.User == "" {
		return fmt.Errorf("Missing auth user")
	}

	i, ok := r.items[identifier]
	if !ok {
		return errClientNotSubscribed
	}

	if _, ok := r.hosts[host]; ok {
		return fmt.Errorf("host %q is occupied", host)
	}
	r.hosts[host] = &hostInfo{
		identifier: identifier,
		auth:       auth,
	}

	i.Hosts = append(i.Hosts, host)

	return nil
}

// DeleteHost unassigns host from client.
func (r *registry) DeleteHost(hostPort string, identifier id.ID) {
	host := trimPort(hostPort)

	r.mu.Lock()
	defer r.mu.Unlock()

	if h, ok := r.hosts[host]; !ok || h.identifier != identifier {
		return
	}

	delete(r.hosts, host)

	i := r.items[identifier]
	for k, v := range i.Hosts {
		if v == host {
			i.Hosts = append(i.Hosts[:k], i.Hosts[k+1:]...)
			return
		}
	}
}

// AddListener adds client listener.
func (r *registry) AddListener(l net.Listener, identifier id.ID) error {
	if l == nil {
		panic("Missing listener")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[identifier]
	if !ok {
		return errClientNotSubscribed
	}

	for k, v := range i.Listeners {
		if v == l {
			return fmt.Errorf("listener already added at %d", k)
		}
	}

	i.Listeners = append(i.Listeners, l)

	return nil
}

// DeleteListener removes listener from client. Listener must be closed to stop
// accepting go routine.
func (r *registry) DeleteListener(l net.Listener, identifier id.ID) {
	if l == nil {
		panic("Missing listener")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[identifier]
	if !ok {
		return
	}

	for k, v := range i.Listeners {
		if v == l {
			i.Listeners = append(i.Listeners[:k], i.Listeners[k+1:]...)
			return
		}
	}
}

func trimPort(hostPort string) (host string) {
	host, _, _ = net.SplitHostPort(hostPort)
	if host == "" {
		host = hostPort
	}
	return
}
