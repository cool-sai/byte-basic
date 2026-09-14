package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebCacheControl(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/", "no-cache"},
		{"/index.html", "no-cache"},
		{"/scm", "no-cache"},
		{"/assets/index-M1vDZPvf.js", "public, max-age=31536000, immutable"},
		{"/assets/index-DUAoBKAe.css", "public, max-age=31536000, immutable"},
	}
	for _, c := range cases {
		if g := webCacheControl(c.path); g != c.want {
			t.Fatalf("%s: got %q want %q", c.path, g, c.want)
		}
	}
}

func TestWithMinikite(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
	})
	h := withMinikite(inner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/minikitex/api/login", nil))
	if g := rec.Header().Get("X-Path"); g != "/api/login" {
		t.Fatalf("stripped=%q", g)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/login", nil))
	if g := rec.Header().Get("X-Path"); g != "/api/login" {
		t.Fatalf("plain=%q", g)
	}
}
