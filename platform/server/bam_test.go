package main

import (
	"os"
	"path/filepath"
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
}
