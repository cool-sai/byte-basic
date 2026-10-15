package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenderTlbStripsSitePrefix(t *testing.T) {
	conf := renderTlbNginx([]tlbSiteFull{{
		tlbSite: tlbSite{Host: "console.example.com"},
		routes: []tlbRoute{
			{Path: "/minikitex/platform.v1.PlatformService", Target: "platform:8081"},
			{Path: "/minikitex/api", Target: "platform:8081"},
			{Path: "/minikitex", Target: "platform-web:80"},
			{Path: "/api", Target: "home:80"},
			{Path: "/home.v1.HomeService", Target: "home:80"},
			{Path: "/", Target: "home-web:80"},
		},
	}})
	for _, path := range []string{
		"/minikitex/platform.v1.PlatformService",
		"/minikitex/api",
		"/minikitex",
		"/api",
		"/home.v1.HomeService",
	} {
		block := "location " + path + " {"
		i := strings.Index(conf, block)
		if i < 0 {
			t.Fatalf("missing %s", path)
		}
		rest := conf[i:]
		j := strings.Index(rest, "proxy_pass")
		want := "rewrite ^" + regexp.QuoteMeta(path) + "/?(.*)$ /$1 break;"
		if j < 0 || !strings.Contains(rest[:j], want) {
			t.Fatalf("%s did not strip the configured prefix", path)
		}
	}
	block := "location / {"
	i := strings.Index(conf, block)
	if i < 0 {
		t.Fatal("missing /")
	}
	rest := conf[i:]
	j := strings.Index(rest, "proxy_pass")
	if j < 0 || strings.Contains(rest[:j], "rewrite") {
		t.Fatal("/ should forward the original path")
	}
}

func TestTlbSiteFromInput(t *testing.T) {
	t.Setenv("TLB_ZONE", "")
	t.Setenv("TLB_TLS_NAME", "")
	cases := []struct {
		name, host, wantName, wantHost string
	}{
		{"order", "", "order", "order.ls-byte-basic.com"},
		{"", "order", "order", "order.ls-byte-basic.com"},
		{"", "lssl.work", "lssl", "lssl.work"},
		{"console", "lssl.work", "console", "lssl.work"},
		{"", "lssl.work www.lssl.work", "lssl", "lssl.work www.lssl.work"},
	}
	for _, c := range cases {
		n, h, err := tlbSiteFromInput(c.name, c.host)
		if err != nil {
			t.Fatalf("%q %q: %v", c.name, c.host, err)
		}
		if n != c.wantName || h != c.wantHost {
			t.Fatalf("%q %q: got %s %s want %s %s", c.name, c.host, n, h, c.wantName, c.wantHost)
		}
	}
}

func TestSeedGitURL(t *testing.T) {
	t.Setenv("SCM_GIT_URL", "https://example.com/x.git")
	if g := seedGitURL("."); g != "https://example.com/x.git" {
		t.Fatalf("got %s", g)
	}
}

func TestComposeServicePorts(t *testing.T) {
	raw := `{
		"services": {
			"board": {
				"environment": {"LISTEN": "0.0.0.0:8090"},
				"expose": ["8090"]
			},
			"order": {
				"environment": {"LISTEN": "0.0.0.0:8889", "METRICS": "0.0.0.0:9091"},
				"expose": ["9091"],
				"ports": [{"target": 8889}, {"target": 8080}]
			},
			"web": {"expose": ["80/tcp"]},
			"tlb": {"expose": ["80"]}
		}
	}`
	got := composeServicePorts(raw)
	if strings.Join(got["board"], ",") != "8090" {
		t.Fatalf("board %v", got["board"])
	}
	if strings.Join(got["order"], ",") != "8889,8080" {
		t.Fatalf("order %v", got["order"])
	}
	if strings.Join(got["web"], ",") != "80" {
		t.Fatalf("web %v", got["web"])
	}
	if _, ok := got["tlb"]; !ok {
		t.Fatal("parser should still return tlb; skip is ListTlbUpstreams")
	}
}

func TestTlbZoneFromTLS(t *testing.T) {
	t.Setenv("TLB_ZONE", "")
	t.Setenv("TLB_TLS_NAME", "lssl.work")
	if g := tlbZone(); g != "lssl.work" {
		t.Fatalf("zone=%q", g)
	}
	n, h, err := tlbSiteFromInput("order", "")
	if err != nil {
		t.Fatal(err)
	}
	if n != "order" || h != "order.lssl.work" {
		t.Fatalf("got %s %s", n, h)
	}
}

func writeTlbCert(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"fullchain.pem", "privkey.pem"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func siteBlock(t *testing.T, conf, host string) string {
	t.Helper()
	i := strings.Index(conf, "server_name "+host+";")
	if i < 0 {
		t.Fatalf("missing server %s", host)
	}
	rest := conf[i:]
	if j := strings.Index(rest[1:], "server_name "); j >= 0 {
		rest = rest[:j+1]
	}
	return rest
}

func TestRenderTlbPerSiteCert(t *testing.T) {
	root := t.TempDir()
	old := tlbLiveRoot
	tlbLiveRoot = root
	t.Cleanup(func() { tlbLiveRoot = old })
	t.Setenv("TLB_TLS_NAME", "lssl.work")
	writeTlbCert(t, root, "lssl.work")
	writeTlbCert(t, root, "news.lssl.work")

	conf := renderTlbNginx([]tlbSiteFull{
		{tlbSite: tlbSite{Host: "lssl.work www.lssl.work"}, routes: []tlbRoute{{Path: "/", Target: "home-web:80"}}},
		{tlbSite: tlbSite{Host: "news.lssl.work"}, routes: []tlbRoute{{Path: "/", Target: "daily-news:80"}}},
		{tlbSite: tlbSite{Host: "img.lssl.work"}, routes: []tlbRoute{{Path: "/", Target: "common-static:8092"}}},
	})
	cases := map[string]string{
		"lssl.work www.lssl.work": root + "/lssl.work/",
		"news.lssl.work":          root + "/news.lssl.work/",
		"img.lssl.work":           root + "/lssl.work/", // 没有自己的证书，回落默认
	}
	for host, want := range cases {
		blk := siteBlock(t, conf, host)
		if !strings.Contains(blk, "ssl_certificate "+want+"fullchain.pem;") ||
			!strings.Contains(blk, "ssl_certificate_key "+want+"privkey.pem;") {
			t.Fatalf("%s: want cert %s, got\n%s", host, want, blk)
		}
	}
	if !strings.Contains(conf, "listen 443 ssl default_server;") {
		t.Fatal("first site should be 443 default_server")
	}
}

func TestRenderTlbNoTLSWithoutDefaultCert(t *testing.T) {
	root := t.TempDir()
	old := tlbLiveRoot
	tlbLiveRoot = root
	t.Cleanup(func() { tlbLiveRoot = old })
	t.Setenv("TLB_TLS_NAME", "lssl.work")
	writeTlbCert(t, root, "news.lssl.work") // 只有站点证书、没有默认证书：保持旧行为，不开 TLS
	conf := renderTlbNginx([]tlbSiteFull{
		{tlbSite: tlbSite{Host: "news.lssl.work"}, routes: []tlbRoute{{Path: "/", Target: "daily-news:80"}}},
	})
	if strings.Contains(conf, "ssl_certificate") || strings.Contains(conf, "443") {
		t.Fatal("TLS should stay off without default cert")
	}
}

func TestTlbSiteTLSDirRejectsOddHosts(t *testing.T) {
	root := t.TempDir()
	old := tlbLiveRoot
	tlbLiveRoot = root
	t.Cleanup(func() { tlbLiveRoot = old })
	t.Setenv("TLB_TLS_NAME", "lssl.work")
	writeTlbCert(t, root, "lssl.work")
	for _, h := range []string{"", "_", "../lssl.work", "*.lssl.work"} {
		if got := tlbSiteTLSDir(h); got != root+"/lssl.work" {
			t.Fatalf("%q -> %s", h, got)
		}
	}
}
