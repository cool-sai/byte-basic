package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var jobName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type scmJob struct {
	Name       string
	GitURL     string
	ScriptPath string
}

func (s *server) listJobs(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`SELECT id, name, repo_dir, script_path, created_at FROM scm_job ORDER BY id DESC`)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	jobs := scanRows(rows, "id", "name", "gitUrl", "scriptPath", "createdAt")
	if jobs == nil {
		jobs = []map[string]any{}
	}
	writeJSON(w, map[string]any{"jobs": jobs})
}

func (s *server) createJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		GitURL     string `json:"gitUrl"`
		ScriptPath string `json:"scriptPath"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if !jobName.MatchString(name) {
		fail(w, 400, fmt.Errorf("bad scm name"))
		return
	}
	gitURL := strings.TrimSpace(body.GitURL)
	if gitURL == "" {
		fail(w, 400, fmt.Errorf("git url required"))
		return
	}
	scriptPath := strings.TrimSpace(body.ScriptPath)
	if err := checkScriptRel(scriptPath); err != nil {
		fail(w, 400, err)
		return
	}
	out, err := exec.Command("git", "ls-remote", "--exit-code", gitURL, "HEAD").CombinedOutput()
	if err != nil {
		fail(w, 400, fmt.Errorf("git ls-remote: %w\n%s", err, out))
		return
	}
	_, err = s.db.Exec(`INSERT INTO scm_job (name, repo_dir, script_path) VALUES (?,?,?)`, name, gitURL, scriptPath)
	if err != nil {
		fail(w, 400, fmt.Errorf("create scm: %w", err))
		return
	}
	writeJSON(w, map[string]any{"name": name, "gitUrl": gitURL, "scriptPath": scriptPath})
}

func (s *server) getJob(name string) (scmJob, error) {
	var j scmJob
	err := s.db.QueryRow(
		`SELECT name, repo_dir, script_path FROM scm_job WHERE name=?`, name,
	).Scan(&j.Name, &j.GitURL, &j.ScriptPath)
	if err == sql.ErrNoRows {
		return j, fmt.Errorf("unknown scm %s", name)
	}
	return j, err
}

func checkScriptRel(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return fmt.Errorf("script path must be relative to repo")
	}
	return nil
}

func scriptInRepo(repo, rel string) (string, error) {
	if err := checkScriptRel(rel); err != nil {
		return "", err
	}
	p := filepath.Join(repo, filepath.Clean(rel))
	st, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("script not found: %s", rel)
	}
	if st.IsDir() {
		return "", fmt.Errorf("script is a directory: %s", rel)
	}
	return p, nil
}

type sseLog struct {
	w     http.ResponseWriter
	flush http.Flusher
	buf   strings.Builder
}

func startSSE(w http.ResponseWriter) (*sseLog, error) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("stream not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	return &sseLog{w: w, flush: fl}, nil
}

func (l *sseLog) Write(p []byte) (int, error) {
	text := strings.ReplaceAll(string(p), "\r", "\n")
	l.buf.WriteString(text)
	b, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintf(l.w, "event: log\ndata: %s\n\n", b); err != nil {
		return 0, err
	}
	l.flush.Flush()
	return len(p), nil
}

func (l *sseLog) done(v map[string]any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(l.w, "event: done\ndata: %s\n\n", b)
	l.flush.Flush()
}

func (l *sseLog) String() string {
	return l.buf.String()
}

func streamGit(w io.Writer, dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

func (s *server) gitSync(name, url string, w io.Writer) (string, error) {
	work := filepath.Join(s.root, "scm-work", name)
	if _, err := os.Stat(filepath.Join(work, ".git")); err == nil {
		fmt.Fprint(w, "$ git fetch origin\n")
		if err := streamGit(w, work, "fetch", "--progress", "origin"); err != nil {
			return "", fmt.Errorf("git fetch: %w", err)
		}
		fmt.Fprint(w, "$ git reset --hard origin/HEAD\n")
		if err := streamGit(w, work, "reset", "--hard", "origin/HEAD"); err != nil {
			return "", fmt.Errorf("git pull: %w", err)
		}
		return work, nil
	}
	if err := os.MkdirAll(filepath.Join(s.root, "scm-work"), 0o755); err != nil {
		return "", err
	}
	fmt.Fprintf(w, "$ git clone --progress %s\n", url)
	if err := streamGit(w, "", "clone", "--progress", url, work); err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}
	return work, nil
}

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

func (s *server) getBuild(w http.ResponseWriter, r *http.Request) {
	var id int64
	var service, version, binPath, status, logText string
	var created time.Time
	err := s.db.QueryRow(
		`SELECT id, service, version, bin_path, status, log_text, created_at FROM scm_build WHERE id=?`,
		r.PathValue("id"),
	).Scan(&id, &service, &version, &binPath, &status, &logText, &created)
	if err == sql.ErrNoRows {
		fail(w, 404, fmt.Errorf("build not found"))
		return
	}
	if err != nil {
		fail(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{
		"id": id, "service": service, "version": version, "binPath": binPath,
		"status": status, "log": logText, "createdAt": created.Format(time.RFC3339),
	})
}

func (s *server) createBuild(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	job, err := s.getJob(body.Name)
	if err != nil {
		fail(w, 400, err)
		return
	}
	lg, err := startSSE(w)
	if err != nil {
		fail(w, 500, err)
		return
	}
	repo, err := s.gitSync(job.Name, job.GitURL, lg)
	if err != nil {
		fmt.Fprintf(lg, "%s\n", err)
		lg.done(map[string]any{"status": "fail", "error": err.Error()})
		return
	}
	if _, err := scriptInRepo(repo, job.ScriptPath); err != nil {
		fmt.Fprintf(lg, "%s\n", err)
		lg.done(map[string]any{"status": "fail", "error": err.Error()})
		return
	}
	ver := time.Now().Format("20060102-150405")
	outDir := filepath.Join(s.root, "artifacts", job.Name, ver)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(lg, "%s\n", err)
		lg.done(map[string]any{"status": "fail", "error": err.Error()})
		return
	}
	binPath := filepath.Join(outDir, job.Name)
	arch := dockerArch()
	fmt.Fprintf(lg, "$ sh %s\n(dir=%s out=%s)\n", job.ScriptPath, repo, outDir)
	cmd := exec.Command("/bin/sh", job.ScriptPath)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH="+arch,
		"SCM_NAME="+job.Name,
		"SCM_VERSION="+ver,
		"SCM_OUT="+outDir,
		"SCM_REPO="+repo,
	)
	cmd.Stdout = lg
	cmd.Stderr = lg
	err = cmd.Run()
	status := "ok"
	if err != nil {
		status = "fail"
		fmt.Fprintf(lg, "%s\n", err)
	} else if _, stErr := os.Stat(binPath); stErr != nil {
		status = "fail"
		err = fmt.Errorf("script ok but missing %s; write the binary to $SCM_OUT/$SCM_NAME", binPath)
		fmt.Fprintf(lg, "%s\n", err)
	}
	_, dbErr := s.db.Exec(
		`INSERT INTO scm_build (service, version, bin_path, status, log_text) VALUES (?,?,?,?,?)`,
		job.Name, ver, binPath, status, clip(lg.String()),
	)
	if dbErr != nil {
		lg.done(map[string]any{"status": "fail", "error": dbErr.Error()})
		return
	}
	if err != nil {
		lg.done(map[string]any{"status": status, "service": job.Name, "version": ver, "error": err.Error()})
		return
	}
	lg.done(map[string]any{"status": status, "service": job.Name, "version": ver, "binPath": binPath})
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
