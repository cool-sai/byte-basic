package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeFromArtifactAndPin(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "projects", "board")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	srcY := []byte(`services:
  board:
    build:
      context: ../..
      dockerfile: projects/board/Dockerfile
    image: minikitex-board:old
    environment:
      LISTEN: "0.0.0.0:8090"
      BOARD_TOKEN: ${BOARD_TOKEN}
    expose:
      - "8090"
volumes:
  board-data:
`)
	if err := os.WriteFile(filepath.Join(proj, "compose.yml"), srcY, 0o644); err != nil {
		t.Fatal(err)
	}
	tar := filepath.Join(root, "art.tar")
	cmd := exec.Command("tar", "-cf", tar, "-C", root, "projects/board/compose.yml")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	raw, err := composeFromArtifact(tar, "board")
	if err != nil {
		t.Fatal(err)
	}
	got, err := pinRuntimeImage(raw, "board", "minikitex-board:local")
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, "build:") {
		t.Fatalf("build left:\n%s", s)
	}
	if !strings.Contains(s, "minikitex-board:local") || !strings.Contains(s, "pull_policy") {
		t.Fatalf("%s", s)
	}
	if !strings.Contains(s, "8090") || !strings.Contains(s, "board-data") {
		t.Fatalf("%s", s)
	}
	if _, err := composeFromArtifact(tar, "nope"); err == nil {
		t.Fatal("missing service")
	}
}

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
