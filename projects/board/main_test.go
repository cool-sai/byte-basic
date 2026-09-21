package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func testMux(t *testing.T, token string) http.Handler {
	t.Helper()
	dir := t.TempDir()
	ts, err := NewTokenStore(filepath.Join(dir, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	return newMux(token, newMemBoard(), ts)
}

func TestHealthAndEmptyStats(t *testing.T) {
	ts := httptest.NewServer(testMux(t, "secret"))
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/board/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("health %d", res.StatusCode)
	}
	res, err = http.Get(ts.URL + "/board/api/kv/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("stats %d", res.StatusCode)
	}
}

func TestMCPUnauthorized(t *testing.T) {
	ts := httptest.NewServer(testMux(t, "secret"))
	t.Cleanup(ts.Close)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_items","arguments":{"kind":"算法"}}}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/board/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("no token: %d", res.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/board/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer wrong")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("bad token: %d", res.StatusCode)
	}
}

func TestMCPUpdateThenRead(t *testing.T) {
	ts := httptest.NewServer(testMux(t, "secret"))
	t.Cleanup(ts.Close)
	call := func(name, args string) map[string]any {
		t.Helper()
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/board/mcp", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer secret")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("%s status %d body %s", name, res.StatusCode, b)
		}
		var msg map[string]any
		if err := json.Unmarshal(b, &msg); err != nil {
			t.Fatalf("json %s: %v\n%s", name, err, b)
		}
		return msg
	}
	if msg := call("create_kind", `{"name":"算法"}`); msg["error"] != nil {
		t.Fatalf("create_kind %+v", msg["error"])
	}
	add := call("add_item", `{"kind":"算法","content":"两数之和","date":"2026-09-21","type":"数组"}`)
	if add["error"] != nil {
		t.Fatalf("add_item %+v", add["error"])
	}
	res, err := http.Get(ts.URL + "/board/api/kv/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("kv %d", res.StatusCode)
	}
	var sum Summary
	if err := json.NewDecoder(res.Body).Decode(&sum); err != nil {
		t.Fatal(err)
	}
	if sum.Total != 1 || len(sum.Kinds) != 1 || sum.Kinds[0].Count != 1 {
		t.Fatalf("kv %+v", sum)
	}
	listed := call("list_items", `{"kind":"算法"}`)
	if listed["error"] != nil {
		t.Fatalf("list_items %+v", listed["error"])
	}
}
