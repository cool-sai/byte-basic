package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "minikitex/gen/platform/v1"
)

func TestCompileBamPlatform(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "proto/platform/v1/platform.proto"))
	if err != nil {
		t.Fatal(err)
	}
	rpcs, httpN, perr := compileBam(root, []*v1.BamFile{
		{Path: "platform/v1/platform.proto", Content: string(b)},
	})
	if perr != "" {
		t.Fatal(perr)
	}
	if len(rpcs) == 0 {
		t.Fatal("no rpcs")
	}
	if httpN == 0 {
		t.Fatal("no http annotations")
	}
	var login *v1.BamRpc
	for _, r := range rpcs {
		if r.Name == "Login" {
			login = r
			break
		}
	}
	if login == nil {
		t.Fatal("missing Login")
	}
	if login.HttpMethod != "POST" || login.Uri != "/api/login" {
		t.Fatalf("login http %s %s", login.HttpMethod, login.Uri)
	}
}

func TestProtoModuleRoot(t *testing.T) {
	if g := protoModuleRoot("", "proto/platform/v1"); g != "proto" {
		t.Fatalf("got %s", g)
	}
	if g := protoModuleRoot("", "services/order"); g != "services/order" {
		t.Fatalf("got %s", g)
	}
}

func TestCollectBamFiles(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, err := collectBamFiles(root, "proto/platform/v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files")
	}
	if files[0].Path != "platform/v1/platform.proto" {
		t.Fatalf("path %s", files[0].Path)
	}
}

func TestBamZipName(t *testing.T) {
	name, ok := bamZipName("/api/bam/modules/platform/download.zip")
	if !ok || name != "platform" {
		t.Fatalf("%s %v", name, ok)
	}
	if _, ok := bamZipName("/api/bam/modules/platform/download"); ok {
		t.Fatal("expected miss")
	}
	name, ok = bamPullName("/api/bam/modules/platform.api/pull.zip")
	if !ok || name != "platform.api" {
		t.Fatalf("%s %v", name, ok)
	}
}

func TestFilterZipLang(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{"platform/v1/a.go", "ts/platform/v1/a.ts"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	goz, err := filterZipLang(buf.Bytes(), "go")
	if err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, goz)
	if len(names) != 1 || names[0] != "platform/v1/a.go" {
		t.Fatalf("%v", names)
	}
	tsz, err := filterZipLang(buf.Bytes(), "ts")
	if err != nil {
		t.Fatal(err)
	}
	names = zipNames(t, tsz)
	if len(names) != 1 || names[0] != "platform/v1/a.ts" {
		t.Fatalf("%v", names)
	}
}

func zipNames(t *testing.T, b []byte) []string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range r.File {
		out = append(out, f.Name)
	}
	return out
}

func TestParseGitLsRemote(t *testing.T) {
	def, br := parseGitLsRemote("ref: refs/heads/main\tHEAD\nabc\tHEAD\nabc\trefs/heads/main\ndef\trefs/heads/feat\n")
	if def != "main" || len(br) != 2 {
		t.Fatalf("%s %+v", def, br)
	}
	if br[0].Name != "main" || br[0].Commit != "abc" || br[1].Name != "feat" {
		t.Fatalf("%+v", br)
	}
}

func TestNormalizeGitURL(t *testing.T) {
	a := normalizeGitURL("https://github.com/cool-sai/byte-basic.git")
	b := normalizeGitURL("git@github.com:cool-sai/byte-basic.git")
	if a != b || a != "github.com/cool-sai/byte-basic" {
		t.Fatalf("%s %s", a, b)
	}
}

func TestParseGitHook(t *testing.T) {
	br, commit, url := parseGitHook([]byte(`{"ref":"refs/heads/main","after":"abc123","repository":{"clone_url":"https://github.com/a/b.git"}}`))
	if br != "main" || commit != "abc123" || !strings.Contains(url, "github.com/a/b") {
		t.Fatalf("%s %s %s", br, commit, url)
	}
	br, commit, url = parseGitHook([]byte(`{"ref":"refs/heads/main","after":"0000","deleted":true}`))
	if br != "" {
		t.Fatalf("deleted %s %s %s", br, commit, url)
	}
}
