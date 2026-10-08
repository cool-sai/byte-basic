package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKindsPageAndItemsAPI(t *testing.T) {
	h := testMux(t, "secret")
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/kinds/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), "种类") {
		t.Fatalf("page %d %s", res.StatusCode, b)
	}

	res, err = http.Get(ts.URL + "/api/items")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("missing kind %d", res.StatusCode)
	}

	call := func(name, args string) {
		t.Helper()
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer secret")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("%s %d", name, r.StatusCode)
		}
	}
	call("create_kind", `{"name":"算法"}`)
	call("add_item", `{"kind":"算法","content":"题1","date":"2026-09-21"}`)
	call("add_item", `{"kind":"算法","content":"题2","date":"2026-09-20"}`)

	res, err = http.Get(ts.URL + "/api/items?kind=%E7%AE%97%E6%B3%95&page=1&page_size=1")
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("items %d %s", res.StatusCode, b)
	}
	var page ListPage
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Page != 1 || len(page.Items) != 1 {
		t.Fatalf("page %+v", page)
	}
	if page.Items[0].Content != "题1" {
		t.Fatalf("newest first %+v", page.Items[0])
	}

	res, err = http.Get(ts.URL + "/api/items?kind=nope")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("missing kind %d", res.StatusCode)
	}
}
