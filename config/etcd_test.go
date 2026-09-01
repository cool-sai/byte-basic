package config

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetPut(t *testing.T) {
	store := map[string]string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/kv/put", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Key, Value string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		k, _ := base64.StdEncoding.DecodeString(req.Key)
		v, _ := base64.StdEncoding.DecodeString(req.Value)
		store[string(k)] = string(v)
		_, _ = io.WriteString(w, `{"header":{"revision":"2"}}`)
	})
	mux.HandleFunc("/v3/kv/range", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Key      string `json:"key"`
			RangeEnd string `json:"range_end"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.RangeEnd != "" {
			var kvs []map[string]string
			for k, v := range store {
				kvs = append(kvs, map[string]string{
					"key":   base64.StdEncoding.EncodeToString([]byte(k)),
					"value": base64.StdEncoding.EncodeToString([]byte(v)),
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"header": map[string]string{"revision": "2"},
				"kvs":    kvs,
			})
			return
		}
		k, _ := base64.StdEncoding.DecodeString(req.Key)
		v, ok := store[string(k)]
		if !ok {
			_, _ = io.WriteString(w, `{"header":{"revision":"2"}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"header": map[string]string{"revision": "2"},
			"kvs": []map[string]string{{
				"key":   req.Key,
				"value": base64.StdEncoding.EncodeToString([]byte(v)),
			}},
		})
	})
	mux.HandleFunc("/v3/kv/deleterange", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Key string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		k, _ := base64.StdEncoding.DecodeString(req.Key)
		delete(store, string(k))
		_, _ = io.WriteString(w, `{"header":{"revision":"3"}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	val, ok, _, err := Get(srv.URL, "user/name_suffix")
	if err != nil || ok {
		t.Fatalf("empty get val=%q ok=%v err=%v", val, ok, err)
	}
	if err := Put(srv.URL, "user/name_suffix", "!!!"); err != nil {
		t.Fatal(err)
	}
	val, ok, _, err = Get(srv.URL, "user/name_suffix")
	if err != nil || !ok || val != "!!!" {
		t.Fatalf("val=%q ok=%v err=%v", val, ok, err)
	}

	listed, err := List(srv.URL)
	if err != nil || len(listed) != 1 || listed[0] != (KV{Key: "user/name_suffix", Value: "!!!"}) {
		t.Fatalf("list=%v err=%v", listed, err)
	}
	if err := Delete(srv.URL, "user/name_suffix"); err != nil {
		t.Fatal(err)
	}
	listed, err = List(srv.URL)
	if err != nil || len(listed) != 0 {
		t.Fatalf("after delete list=%v err=%v", listed, err)
	}
}

func TestWatchUpdatesVar(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/kv/range", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"header":{"revision":"1"}}`)
	})
	mux.HandleFunc("/v3/watch", func(w http.ResponseWriter, r *http.Request) {
		fl, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, `{"result":{"events":[{"type":"PUT","kv":{"value":"`+base64.StdEncoding.EncodeToString([]byte("!!!"))+`"}}]}}`+"\n")
		if fl != nil {
			fl.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	v := NewVar("")
	done := make(chan struct{})
	go func() {
		_ = v.sync(srv.URL, "user/name_suffix")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if v.Get() != "!!!" {
		t.Fatalf("got %q", v.Get())
	}
}
