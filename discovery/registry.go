package discovery

import (
	"sort"
	"sync"
	"time"
)

// Registry is a tiny in-memory name → addrs map with TTL.
// Consul/etcd do the same job with persistence, health checks, and watches.
type Registry struct {
	ttl time.Duration
	mu  sync.Mutex
	// name -> addr -> expiry
	inst map[string]map[string]time.Time
}

func NewRegistry(ttl time.Duration) *Registry {
	if ttl <= 0 {
		ttl = 8 * time.Second
	}
	return &Registry{ttl: ttl, inst: map[string]map[string]time.Time{}}
}

func (r *Registry) Register(name, addr string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.inst[name]
	if m == nil {
		m = map[string]time.Time{}
		r.inst[name] = m
	}
	m[addr] = now.Add(r.ttl)
}

func (r *Registry) Lookup(name string, now time.Time) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.inst[name]
	var addrs []string
	for addr, exp := range m {
		if now.After(exp) {
			delete(m, addr)
			continue
		}
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	return addrs
}
