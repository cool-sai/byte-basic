package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type bamFile struct {
	Endpoint string      `yaml:"endpoint"`
	Modules  []bamModule `yaml:"modules"`
}

type bamModule struct {
	Name    string `yaml:"name"`
	Out     string `yaml:"out"`
	Version int    `yaml:"version"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "update":
		err = runUpdate(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: bam update

bam.yaml 放在项目根目录：

endpoint: http://127.0.0.1:8081
modules:
  - name: platform.api
    out: gen
`)
}

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	cfgPath := fs.String("f", "", "bam.yaml 路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, root, err := loadConfigPath(*cfgPath)
	if err != nil {
		return err
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("bam.yaml: missing endpoint")
	}
	if len(cfg.Modules) == 0 {
		return fmt.Errorf("bam.yaml: no modules")
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	ep := strings.TrimRight(cfg.Endpoint, "/")
	for _, m := range cfg.Modules {
		if m.Name == "" {
			return fmt.Errorf("module missing name")
		}
		outDir := m.Out
		if outDir == "" {
			outDir = "gen"
		}
		if !filepath.IsAbs(outDir) {
			outDir = filepath.Join(root, outDir)
		}
		n, err := pullModule(client, ep, m, outDir)
		if err != nil {
			return fmt.Errorf("%s: %w", m.Name, err)
		}
		fmt.Printf("%s -> %s (%d files)\n", m.Name, outDir, n)
	}
	return nil
}

func pullModule(client *http.Client, ep string, m bamModule, outDir string) (int, error) {
	u := ep + "/api/bam/modules/" + m.Name + "/download.zip"
	if m.Version > 0 {
		u += fmt.Sprintf("?version=%d", m.Version)
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != 200 {
		return 0, fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(raw)))
	}
	return extractZip(raw, outDir)
}

func extractZip(zipb []byte, dest string) (int, error) {
	r, err := zip.NewReader(bytes.NewReader(zipb), int64(len(zipb)))
	if err != nil {
		return 0, err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range r.File {
		rel := filepath.FromSlash(f.Name)
		if rel == "" || rel == "." || strings.Contains(rel, "..") {
			return n, fmt.Errorf("bad path %s", f.Name)
		}
		path := filepath.Join(dest, rel)
		if path != dest && !strings.HasPrefix(path, dest+string(os.PathSeparator)) {
			return n, fmt.Errorf("bad path %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return n, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return n, err
		}
		rc, err := f.Open()
		if err != nil {
			return n, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return n, err
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func loadConfigPath(explicit string) (*bamFile, string, error) {
	var path string
	if explicit != "" {
		path = explicit
	} else {
		dir, err := os.Getwd()
		if err != nil {
			return nil, "", err
		}
		for {
			p := filepath.Join(dir, "bam.yaml")
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				return nil, "", fmt.Errorf("no bam.yaml")
			}
			dir = parent
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	cfg, err := parseConfig(b)
	if err != nil {
		return nil, "", err
	}
	return cfg, filepath.Dir(path), nil
}

func parseConfig(b []byte) (*bamFile, error) {
	var cfg bamFile
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
