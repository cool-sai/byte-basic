package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pagesOnlyServer mounts pagesHandler alone — production Host isolation is TLB → pages-web;
// these tests cover nav / uploaded-home behavior of the leftover handler.
func pagesOnlyServer(t *testing.T, cfg contentConfig) (*httptest.Server, *memContent) {
	t.Helper()
	c := newMemContent()
	srv := httptest.NewServer(pagesHandler(c, cfg))
	t.Cleanup(srv.Close)
	return srv, c
}

func navGet(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return res, string(b)
}

func TestNavEmpty(t *testing.T) {
	srv, _ := pagesOnlyServer(t, contentConfig{PagesHost: "pages.example"})
	res, body := navGet(t, srv.URL+"/")
	if res.StatusCode != 200 || !strings.Contains(body, "还没有任何模块或页面") {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "default-src 'none'") || strings.Contains(body, "<script") {
		t.Fatalf("csp/script: %s", res.Header.Get("Content-Security-Policy"))
	}
}

func TestNavListsModulesAndPages(t *testing.T) {
	srv, c := pagesOnlyServer(t, contentConfig{PagesHost: "pages.example"})
	mustPut(t, c, NodeInput{Type: NodeModule, Slug: "finance", Title: "理财", Data: json.RawMessage(`{"description":"基金<b>触发</b>"}`)})
	mustPut(t, c, NodeInput{Type: NodeModule, Slug: "news", Title: "新闻"})
	if _, err := c.PutPage("finance/demo", map[string]string{"index.html": "demo"}); err != nil {
		t.Fatal(err)
	}
	_, _ = c.PutPage("misc/x", map[string]string{"index.html": "x"})
	_, body := navGet(t, srv.URL+"/")
	for _, want := range []string{"理财", "基金&lt;b&gt;触发&lt;/b&gt;", `href="/finance/demo/"`, "新闻", "暂无页面", "其他页面", `href="/misc/x/"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	if res, b := navGet(t, srv.URL+"/finance/demo/"); res.StatusCode != 200 || b != "demo" {
		t.Fatalf("page %d %q", res.StatusCode, b)
	}
}

func TestNavCannotBeOverriddenByUpload(t *testing.T) {
	srv, c := pagesOnlyServer(t, contentConfig{PagesHost: "pages.example"})
	for _, p := range []string{"", "/", "index.html"} {
		if _, err := c.PutPage(p, map[string]string{"index.html": "evil"}); err == nil && p != "index.html" {
			t.Fatalf("root path %q accepted", p)
		}
	}
	if _, b := navGet(t, srv.URL+"/"); !strings.Contains(b, "页面导航") || strings.Contains(b, "evil") {
		t.Fatalf("nav overridden: %s", b)
	}
}

func TestNavRootServesUploadedHome(t *testing.T) {
	srv, c := pagesOnlyServer(t, contentConfig{PagesHost: "pages.example"})
	if _, err := c.PutPage("home", map[string]string{"index.html": "<p>home</p>", "assets/a.js": "js"}); err != nil {
		t.Fatal(err)
	}
	res, b := navGet(t, srv.URL+"/")
	if res.StatusCode != 200 || b != "<p>home</p>" || strings.Contains(b, "页面导航") {
		t.Fatalf("root %d %q", res.StatusCode, b)
	}
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("csp: %s", res.Header.Get("Content-Security-Policy"))
	}
	if res, b := navGet(t, srv.URL+"/home/assets/a.js"); res.StatusCode != 200 || b != "js" {
		t.Fatalf("asset %d %q", res.StatusCode, b)
	}
}
