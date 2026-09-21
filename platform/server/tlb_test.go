package main

import (
	"strings"
	"testing"
)

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
