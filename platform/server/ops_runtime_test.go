package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeServiceYAML(t *testing.T) {
	node := runtimeServiceYAML("home-web", "minikitex-home-web:local", "node")
	if strings.Contains(node, "LISTEN") {
		t.Fatalf("node should not set LISTEN:\n%s", node)
	}
	if !strings.Contains(node, "  home-web:") {
		t.Fatalf("%s", node)
	}
	goYAML := runtimeServiceYAML("home", "minikitex-home:local", "golang")
	if !strings.Contains(goYAML, "LISTEN: \"0.0.0.0:80\"") {
		t.Fatalf("%s", goYAML)
	}
}

func TestResolveAppCompose(t *testing.T) {
	got, err := resolveAppCompose("user", "")
	if err != nil || strings.Join(got, ",") != "user" {
		t.Fatalf("catalog fallback: %v %v", got, err)
	}
	got, err = resolveAppCompose("home", "")
	if err != nil || strings.Join(got, ",") != "home" {
		t.Fatalf("name fallback: %v %v", got, err)
	}
	got, err = resolveAppCompose("home", " home , home-web ")
	if err != nil || strings.Join(got, ",") != "home,home-web" {
		t.Fatalf("explicit: %v %v", got, err)
	}
	if _, err := resolveAppCompose("home", "bad name"); err == nil {
		t.Fatal("expected bad compose")
	}
}

func TestComposeDropped(t *testing.T) {
	got := composeDropped([]string{"home", "home-web"}, []string{"home"})
	if strings.Join(got, ",") != "home-web" {
		t.Fatalf("%v", got)
	}
	if got := extraCompose([]string{"home", "platform", "bad name"}, map[string]bool{"platform": true}); strings.Join(got, ",") != "home" {
		t.Fatalf("%v", got)
	}
}

func TestSplitCombinedRuntime(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	combined := "services:\n" +
		strings.TrimPrefix(runtimeServiceYAML("home-web", "minikitex-home-web:local", "node"), "services:\n") +
		strings.TrimPrefix(runtimeServiceYAML("home", "minikitex-home:local", "golang"), "services:\n")
	if err := os.WriteFile(filepath.Join(root, "deploy", "runtime.yml"), []byte(combined), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := splitCombinedRuntime(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "deploy", "runtime.yml")); !os.IsNotExist(err) {
		t.Fatalf("combined still there: %v", err)
	}
	web, err := os.ReadFile(filepath.Join(root, "deploy", "runtime", "home-web.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(web), "  home-web:") || strings.Contains(string(web), "  home:") {
		t.Fatalf("home-web.yml:\n%s", web)
	}
	api, err := os.ReadFile(filepath.Join(root, "deploy", "runtime", "home.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(api), "  home:") || !strings.Contains(string(api), "LISTEN") {
		t.Fatalf("home.yml:\n%s", api)
	}
}
