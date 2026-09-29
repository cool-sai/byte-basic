package main

import (
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
