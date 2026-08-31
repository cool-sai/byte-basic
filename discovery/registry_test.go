package discovery

import (
	"sort"
	"testing"
	"time"
)

func TestTTLExpiry(t *testing.T) {
	r := NewRegistry(time.Second)
	now := time.Unix(1000, 0)
	r.Register("user", "user-1:8888", now)
	r.Register("user", "user-2:8888", now)

	got := r.Lookup("user", now)
	sort.Strings(got)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}

	got = r.Lookup("user", now.Add(2*time.Second))
	if len(got) != 0 {
		t.Fatalf("expired still there: %v", got)
	}
}
