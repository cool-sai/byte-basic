package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeComposePSIgnoresWarn(t *testing.T) {
	raw := []byte(`time="2026-09-21T19:00:00Z" level=warning msg="Found orphan containers"
{"ID":"abc","Name":"golang-board-1","Service":"board","State":"running","Status":"Up 2 seconds"}
{"ID":"def","Service":"platform","State":"running"}
`)
	got, err := decodeComposePS(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Service != "board" || got[1].Service != "platform" {
		t.Fatalf("%+v", got)
	}
	arr := []byte(`[{"Service":"tlb","State":"running"}]`)
	got, err = decodeComposePS(arr)
	if err != nil || len(got) != 1 || got[0].Service != "tlb" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestMountSourceFrom(t *testing.T) {
	info := "2073 1650 253:1 /opt/byte-basic /work rw,relatime master:1 - ext4 /dev/vda1 rw\n"
	if got := mountSourceFrom(info, "/work"); got != "/opt/byte-basic" {
		t.Fatalf("got %q", got)
	}
	if got := mountSourceFrom("1 2 0:0 /work /work rw - ext4 /dev/sda rw\n", "/work"); got != "" {
		t.Fatalf("same path %q", got)
	}
}

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

func TestImageArtifactMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "art")
	if err := writeImageArtifactMarker(path, "abc123"); err != nil {
		t.Fatal(err)
	}
	if got := imageArtifactCommit(path); got != "abc123" {
		t.Fatalf("commit %q", got)
	}
	// non-marker (e.g. old tar / binary) -> empty
	if err := os.WriteFile(path, []byte("not a marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := imageArtifactCommit(path); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestSyncHostRootComposeAndWorkOverlay(t *testing.T) {
	host := t.TempDir()
	work := t.TempDir()
	repoCompose := []byte(`services:
  platform:
    image: minikitex-platform:local
    pull_policy: never
`)
	if err := os.WriteFile(filepath.Join(work, "docker-compose.yml"), repoCompose, 0o644); err != nil {
		t.Fatal(err)
	}
	old := []byte("services:\n  platform:\n    image: old\n")
	hostCompose := filepath.Join(host, "docker-compose.yml")
	if err := os.WriteFile(hostCompose, old, 0o644); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	if err := syncHostRootCompose(&log, host, work); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(hostCompose)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, repoCompose) {
		t.Fatalf("host compose not synced: %q", got)
	}
	if !strings.Contains(log.String(), "synced root compose") {
		t.Fatalf("log %q", log.String())
	}
	if _, err := os.Stat(hostCompose + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("unexpected bak: %v", err)
	}
	if _, err := os.Stat(filepath.Join(host, "deploy", "runtime", "_root.yml")); !os.IsNotExist(err) {
		t.Fatal("should not stage _root.yml")
	}

	log.Reset()
	if err := syncHostRootCompose(&log, host, work); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "unchanged") {
		t.Fatalf("expected unchanged, got %q", log.String())
	}

	// golang/node path: no scm-work compose -> leave host alone
	emptyWork := t.TempDir()
	if err := os.WriteFile(hostCompose, old, 0o644); err != nil {
		t.Fatal(err)
	}
	log.Reset()
	if err := syncHostRootCompose(&log, host, emptyWork); err != nil {
		t.Fatal(err)
	}
	hostAfter, _ := os.ReadFile(hostCompose)
	if !bytes.Equal(hostAfter, old) {
		t.Fatalf("host mutated without work compose: %q", hostAfter)
	}
	if log.Len() != 0 {
		t.Fatalf("expected silent skip, got %q", log.String())
	}

	overlay := []byte("services:\n  board:\n    image: x\n    ports: [\"8090:8090\"]\n")
	if err := os.MkdirAll(filepath.Join(work, "projects", "board"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "projects", "board", "compose.yml"), overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := composeFromWork(work, "board")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, overlay) {
		t.Fatalf("overlay %q", raw)
	}
	if _, err := composeFromWork(work, "nope"); err == nil {
		t.Fatal("expected missing service")
	}

	s := &server{root: host}
	args := s.composeFileArgs()
	if len(args) < 2 || args[0] != "-f" || args[1] != "docker-compose.yml" {
		t.Fatalf("composeFileArgs %v", args)
	}
}
