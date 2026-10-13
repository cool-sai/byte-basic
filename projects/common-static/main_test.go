package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPullsMatchingImage(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/img/2463ff1bbdb445f7928e6c4224406156.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(w, "png")
	}))
	defer origin.Close()

	srv := httptest.NewServer(newMux(origin.URL))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/img/2463ff1bbdb445f7928e6c4224406156.png")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" || string(body) != "png" {
		t.Fatalf("got %d %s %q", res.StatusCode, res.Header.Get("Content-Type"), body)
	}

	bad, err := http.Get(srv.URL + "/img/not-a-key.png")
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Body.Close()
	if bad.StatusCode != http.StatusNotFound {
		t.Fatalf("bad key %d", bad.StatusCode)
	}
}

func TestRejectsNonImage(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html>")
	}))
	defer origin.Close()

	srv := httptest.NewServer(newMux(origin.URL))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/img/2463ff1bbdb445f7928e6c4224406156.png")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d", res.StatusCode)
	}
}
