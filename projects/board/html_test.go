package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKindHTMLCRUDAndRender(t *testing.T) {
	ts := httptest.NewServer(testMux(t, "secret"))
	t.Cleanup(ts.Close)

	call := func(name string, args any) map[string]any {
		t.Helper()
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + string(raw) + `}}`
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer secret")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("%s %d %s", name, r.StatusCode, b)
		}
		var msg map[string]any
		if err := json.Unmarshal(b, &msg); err != nil {
			t.Fatalf("json %s: %v\n%s", name, err, b)
		}
		return msg
	}
	failed := func(msg map[string]any) bool {
		if msg["error"] != nil {
			return true
		}
		res, _ := msg["result"].(map[string]any)
		err, _ := res["isError"].(bool)
		return err
	}

	if msg := call("create_kind", map[string]any{"name": "算法"}); failed(msg) {
		t.Fatalf("create_kind %+v", msg)
	}
	if msg := call("create_kind_html", map[string]any{
		"kind": "算法",
		"html": `<article><h3>{{.title}}</h3><p>{{.note}}</p></article>`,
	}); failed(msg) {
		t.Fatalf("create_kind_html %+v", msg)
	}
	if dup := call("create_kind_html", map[string]any{"kind": "算法", "html": `<p>{{.title}}</p>`}); !failed(dup) {
		t.Fatal("duplicate html should fail")
	}
	if msg := call("add_item", map[string]any{
		"kind":    "算法",
		"content": `{"title":"两数之和","note":"<b>hash</b>"}`,
		"date":    "2026-09-21",
	}); failed(msg) {
		t.Fatalf("add_item %+v", msg)
	}

	items := func() ListPage {
		t.Helper()
		res, err := http.Get(ts.URL + "/api/items?kind=" + "%E7%AE%97%E6%B3%95")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("items %d %s", res.StatusCode, b)
		}
		var page ListPage
		if err := json.Unmarshal(b, &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("items %+v", page)
		}
		return page
	}

	got := items().Items[0].Rendered
	if !strings.Contains(got, "<h3>两数之和</h3>") {
		t.Fatalf("rendered %q", got)
	}
	if strings.Contains(got, "<b>hash</b>") {
		t.Fatalf("content should be escaped: %q", got)
	}
	if !strings.Contains(got, "&lt;b&gt;hash&lt;/b&gt;") {
		t.Fatalf("escaped note missing: %q", got)
	}

	if msg := call("update_kind_html", map[string]any{
		"kind": "算法",
		"html": `<p>{{.title}}</p>`,
	}); failed(msg) {
		t.Fatalf("update %+v", msg)
	}
	if got := items().Items[0].Rendered; got != "<p>两数之和</p>" {
		t.Fatalf("updated rendered %q", got)
	}
	if msg := call("get_kind_html", map[string]any{"kind": "算法"}); failed(msg) {
		t.Fatalf("get %+v", msg)
	}
	if msg := call("delete_kind_html", map[string]any{"kind": "算法"}); failed(msg) {
		t.Fatalf("delete %+v", msg)
	}
	if got := items().Items[0].Rendered; got != "" {
		t.Fatalf("rendered after delete %q", got)
	}
}

func TestParseKindHTMLAndApply(t *testing.T) {
	if _, err := parseKindHTML(""); err == nil {
		t.Fatal("empty html")
	}
	if _, err := parseKindHTML("{{"); err == nil {
		t.Fatal("bad template")
	}
	items := []Item{{Content: `{"title":"x"}`}}
	applyHTML(items, `<h3>{{.title}}</h3>`)
	if items[0].Rendered != "<h3>x</h3>" {
		t.Fatalf("render %q", items[0].Rendered)
	}
	plain := []Item{{Content: "裸文本"}}
	applyHTML(plain, `<p>{{.}}</p>`)
	if plain[0].Rendered != "<p>裸文本</p>" {
		t.Fatalf("plain %q", plain[0].Rendered)
	}
}
