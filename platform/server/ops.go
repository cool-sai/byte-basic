package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s *server) listBuilds(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	q := `SELECT id, service, version, bin_path, status, log_text, created_at FROM scm_build`
	var args []any
	if svc != "" {
		q += ` WHERE service=?`
		args = append(args, svc)
	}
	q += ` ORDER BY id DESC LIMIT 50`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	writeJSON(w, scanRows(rows, "id", "service", "version", "binPath", "status", "log", "createdAt"))
}

func (s *server) createBuild(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Service string `json:"service"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	svc, ok := lookup(body.Service)
	if !ok {
		fail(w, 400, fmt.Errorf("unknown service %s", body.Service))
		return
	}
	ver := time.Now().Format("20060102-150405")
	outDir := filepath.Join(s.root, "artifacts", svc.Name, ver)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(w, 500, err)
		return
	}
	binPath := filepath.Join(outDir, svc.Bin)
	arch := dockerArch()
	cmd := exec.Command("go", "build", "-o", binPath, svc.Pkg)
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
	out, err := cmd.CombinedOutput()
	status := "ok"
	if err != nil {
		status = "fail"
	}
	_, dbErr := s.db.Exec(
		`INSERT INTO scm_build (service, version, bin_path, status, log_text) VALUES (?,?,?,?,?)`,
		svc.Name, ver, binPath, status, clip(string(out)+"\n"+errStr(err)),
	)
	if dbErr != nil {
		fail(w, 500, dbErr)
		return
	}
	if err != nil {
		fail(w, 500, fmt.Errorf("build: %w\n%s", err, out))
		return
	}
	writeJSON(w, map[string]any{"service": svc.Name, "version": ver, "binPath": binPath, "status": status})
}

func (s *server) listPublishes(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`SELECT id, idl_name, routes_json, status, log_text, created_at FROM agw_publish ORDER BY id DESC LIMIT 50`)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	writeJSON(w, scanRows(rows, "id", "idlName", "routesJson", "status", "log", "createdAt"))
}

func (s *server) publishAGW(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	path, err := s.idlPath(body.Name)
	if err != nil {
		fail(w, 400, err)
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(w, 404, err)
		return
	}
	view, err := idlView(body.Name, string(raw))
	if err != nil {
		fail(w, 400, err)
		return
	}
	routes, _ := json.Marshal(view["methods"])
	out, err := s.compose("restart", "gateway")
	status := "ok"
	if err != nil {
		status = "fail"
	}
	_, dbErr := s.db.Exec(
		`INSERT INTO agw_publish (idl_name, content, routes_json, status, log_text) VALUES (?,?,?,?,?)`,
		body.Name, string(raw), string(routes), status, clip(out+"\n"+errStr(err)),
	)
	if dbErr != nil {
		fail(w, 500, dbErr)
		return
	}
	if err != nil {
		fail(w, 500, fmt.Errorf("agw publish: %w\n%s", err, out))
		return
	}
	writeJSON(w, map[string]any{"name": body.Name, "status": status, "methods": view["methods"]})
}

func (s *server) listDeploys(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	q := `SELECT id, service, version, status, log_text, created_at FROM deploy_record`
	var args []any
	if svc != "" {
		q += ` WHERE service=?`
		args = append(args, svc)
	}
	q += ` ORDER BY id DESC LIMIT 50`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	writeJSON(w, scanRows(rows, "id", "service", "version", "status", "log", "createdAt"))
}

func (s *server) createDeploy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Service string `json:"service"`
		Version string `json:"version"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	svc, ok := lookup(body.Service)
	if !ok {
		fail(w, 400, fmt.Errorf("unknown service %s", body.Service))
		return
	}
	src := filepath.Join(s.root, "artifacts", svc.Name, body.Version, svc.Bin)
	if _, err := os.Stat(src); err != nil {
		fail(w, 400, fmt.Errorf("artifact not found: %s", src))
		return
	}
	dst := filepath.Join(s.root, "bin", svc.Bin)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fail(w, 500, err)
		return
	}
	if err := copyFile(src, dst); err != nil {
		fail(w, 500, err)
		return
	}
	args := append([]string{"build"}, svc.Compose...)
	buildOut, err := s.compose(args...)
	upOut := ""
	if err == nil {
		upArgs := append([]string{"up", "--no-deps", "-d"}, svc.Compose...)
		upOut, err = s.compose(upArgs...)
	}
	status := "ok"
	if err != nil {
		status = "fail"
	}
	logText := clip(buildOut + "\n" + upOut + "\n" + errStr(err))
	_, dbErr := s.db.Exec(
		`INSERT INTO deploy_record (service, version, status, log_text) VALUES (?,?,?,?)`,
		svc.Name, body.Version, status, logText,
	)
	if dbErr != nil {
		fail(w, 500, dbErr)
		return
	}
	if err != nil {
		fail(w, 500, fmt.Errorf("deploy: %w\n%s", err, logText))
		return
	}
	writeJSON(w, map[string]any{"service": svc.Name, "version": body.Version, "status": status})
}

func (s *server) runtime(w http.ResponseWriter, _ *http.Request) {
	out, err := s.compose("ps", "--format", "json")
	if err != nil {
		fail(w, 500, fmt.Errorf("%w\n%s", err, out))
		return
	}
	dec := json.NewDecoder(strings.NewReader(out))
	var items []any
	for {
		var one any
		if err := dec.Decode(&one); err != nil {
			if err == io.EOF {
				break
			}
			fail(w, 500, err)
			return
		}
		items = append(items, one)
	}
	writeJSON(w, items)
}

func (s *server) compose(args ...string) (string, error) {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = s.root
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func dockerArch() string {
	out, err := exec.Command("docker", "info", "--format", "{{.Architecture}}").Output()
	if err != nil {
		return "arm64"
	}
	a := strings.TrimSpace(string(out))
	if a == "aarch64" || a == "arm64" {
		return "arm64"
	}
	return "amd64"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func scanRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}, keys ...string) []map[string]any {
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(keys))
		ptrs := make([]any, len(keys))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := map[string]any{}
		for i, k := range keys {
			row[k] = stringify(vals[i])
		}
		out = append(out, row)
	}
	return out
}

func stringify(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		return x
	}
}

func clip(s string) string {
	if len(s) > 8000 {
		return s[len(s)-8000:]
	}
	return s
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
