package discovery

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConsulRegisterLookup(t *testing.T) {
	var registered map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/service/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&registered); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/agent/check/pass/service:user-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/health/service/user", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("passing") != "true" {
			t.Fatalf("query %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `[{"Service":{"Address":"user-2","Port":8888}},{"Service":{"Address":"user-1","Port":8888}}]`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if err := Register(srv.URL, "user", "user-1:8888"); err != nil {
		t.Fatal(err)
	}
	if registered["Name"] != "user" || registered["Address"] != "user-1" {
		t.Fatalf("registered=%v", registered)
	}

	addrs, err := Lookup(srv.URL, "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 || addrs[0] != "user-1:8888" || addrs[1] != "user-2:8888" {
		t.Fatalf("addrs=%v", addrs)
	}
}
