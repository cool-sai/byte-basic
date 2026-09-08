package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig([]byte(`
endpoint: http://127.0.0.1:8081
modules:
  - name: platform.api
    out: gen
  - name: order
    out: gen
    version: 3
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != "http://127.0.0.1:8081" || len(cfg.Modules) != 2 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.Modules[0].Name != "platform.api" || cfg.Modules[0].Out != "gen" {
		t.Fatalf("%+v", cfg.Modules[0])
	}
	if cfg.Modules[1].Version != 3 {
		t.Fatalf("%+v", cfg.Modules[1])
	}
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("platform/v1/api.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("package v1\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	n, err := extractZip(buf.Bytes(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("n=%d", n)
	}
	b, err := os.ReadFile(filepath.Join(dir, "platform/v1/api.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "package v1\n" {
		t.Fatalf("%q", b)
	}
	if _, err := extractZip(buf.Bytes(), dir); err != nil {
		t.Fatal(err)
	}
	evil := bytes.NewBuffer(nil)
	ew := zip.NewWriter(evil)
	if _, err := ew.Create("../x.go"); err != nil {
		t.Fatal(err)
	}
	_ = ew.Close()
	if _, err := extractZip(evil.Bytes(), dir); err == nil {
		t.Fatal("expected bad path")
	}
}

func TestLoadConfigPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bam.yaml")
	if err := os.WriteFile(p, []byte("endpoint: http://x\nmodules:\n  - name: a\n    out: gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, root, err := loadConfigPath(p)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir || cfg.Endpoint != "http://x" || cfg.Modules[0].Name != "a" {
		t.Fatalf("%s %+v", root, cfg)
	}
}
