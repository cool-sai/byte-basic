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
	"strconv"
	"strings"
	"sync"
	"time"
)

var jobName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type scmJob struct {
	ID         int64
	Name       string
	GitURL     string
	ScriptPath string
	Branch     string
}

func checkBranch(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("branch required")
	}
	if strings.Contains(s, "..") || strings.HasPrefix(s, "-") || strings.ContainsAny(s, " \t\n~^:?*[\\") {
		return fmt.Errorf("bad branch")
	}
	return nil
}

func (s *server) listJobs(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`SELECT id, name, repo_dir, script_path, branch, created_at FROM scm_job ORDER BY id DESC`)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	jobs := scanRows(rows, "id", "name", "gitUrl", "scriptPath", "branch", "createdAt")
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

func (s *server) showJob(w http.ResponseWriter, r *http.Request) {
	j, err := s.getJob(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	writeJSON(w, map[string]any{"id": j.ID, "name": j.Name, "gitUrl": j.GitURL, "scriptPath": j.ScriptPath, "branch": j.Branch})
}

func (s *server) deleteJob(w http.ResponseWriter, r *http.Request) {
	j, err := s.getJob(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	var used string
	err = s.db.QueryRow(`SELECT name FROM deploy_app WHERE scm_name=? LIMIT 1`, j.Name).Scan(&used)
	if err == nil {
		fail(w, 400, fmt.Errorf("still used by deploy %s", used))
		return
	}
	if err != sql.ErrNoRows {
		fail(w, 500, err)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM scm_build WHERE service=?`, j.Name); err != nil {
		fail(w, 500, err)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM scm_job WHERE name=?`, j.Name); err != nil {
		fail(w, 500, err)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.root, "scm-work", j.Name))
	_ = os.RemoveAll(filepath.Join(s.root, "artifacts", j.Name))
	writeJSON(w, map[string]any{"name": j.Name})
}

func (s *server) getJob(name string) (scmJob, error) {
	var j scmJob
	err := s.db.QueryRow(
		`SELECT id, name, repo_dir, script_path, branch FROM scm_job WHERE name=?`, name,
	).Scan(&j.ID, &j.Name, &j.GitURL, &j.ScriptPath, &j.Branch)
	if err == sql.ErrNoRows {
		return j, fmt.Errorf("unknown scm %s", name)
	}
	return j, err
}

func (s *server) listBranches(w http.ResponseWriter, r *http.Request) {
	job, err := s.getJob(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	cmd := exec.Command("git", "ls-remote", "--symref", job.GitURL, "HEAD", "refs/heads/*")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fail(w, 400, fmt.Errorf("git ls-remote: %w\n%s", err, out))
		return
	}
	def := ""
	var branches []map[string]any
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "ref: ") {
			rest := strings.TrimPrefix(line, "ref: ")
			ref, _, _ := strings.Cut(rest, "\t")
			def = strings.TrimPrefix(ref, "refs/heads/")
			continue
		}
		hash, ref, ok := strings.Cut(line, "\t")
		if !ok || !strings.HasPrefix(ref, "refs/heads/") {
			continue
		}
		branches = append(branches, map[string]any{
			"name":   strings.TrimPrefix(ref, "refs/heads/"),
			"commit": hash,
		})
	}
	if branches == nil {
		branches = []map[string]any{}
	}
	writeJSON(w, map[string]any{"default": def, "branches": branches})
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
	cur   strings.Builder
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
	raw := string(p)
	l.absorb(raw)
	text := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
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

func (l *sseLog) absorb(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r':
			l.cur.Reset()
		case '\n':
			l.buf.WriteString(l.cur.String())
			l.buf.WriteByte('\n')
			l.cur.Reset()
		default:
			l.cur.WriteByte(s[i])
		}
	}
}

func (l *sseLog) done(v map[string]any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(l.w, "event: done\ndata: %s\n\n", b)
	l.flush.Flush()
}

func (l *sseLog) String() string {
	return l.buf.String() + l.cur.String()
}

var lives sync.Map

type liveRun struct {
	mu     sync.Mutex
	buf    strings.Builder
	cur    strings.Builder
	subs   []chan string
	done   bool
	result map[string]any
}

func (l *liveRun) absorb(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r':
			l.cur.Reset()
		case '\n':
			l.buf.WriteString(l.cur.String())
			l.buf.WriteByte('\n')
			l.cur.Reset()
		default:
			l.cur.WriteByte(s[i])
		}
	}
}

func (l *liveRun) Write(p []byte) (int, error) {
	raw := string(p)
	l.mu.Lock()
	l.absorb(raw)
	subs := append([]chan string(nil), l.subs...)
	l.mu.Unlock()
	text := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	for _, ch := range subs {
		select {
		case ch <- text:
		default:
		}
	}
	return len(p), nil
}

func (l *liveRun) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String() + l.cur.String()
}

func (l *liveRun) follow() (string, <-chan string, func()) {
	l.mu.Lock()
	snap := l.buf.String() + l.cur.String()
	if l.done {
		l.mu.Unlock()
		ch := make(chan string)
		close(ch)
		return snap, ch, func() {}
	}
	ch := make(chan string, 64)
	l.subs = append(l.subs, ch)
	l.mu.Unlock()
	return snap, ch, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		out := l.subs[:0]
		for _, c := range l.subs {
			if c != ch {
				out = append(out, c)
			}
		}
		l.subs = out
	}
}

func (l *liveRun) finish(result map[string]any) {
	l.mu.Lock()
	l.done = true
	l.result = result
	subs := l.subs
	l.subs = nil
	l.mu.Unlock()
	for _, ch := range subs {
		close(ch)
	}
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

func (s *server) gitSync(name, url, branch string, w io.Writer) (string, error) {
	work := filepath.Join(s.root, "scm-work", name)
	if _, err := os.Stat(filepath.Join(work, ".git")); err == nil {
		fmt.Fprint(w, "$ git fetch origin\n")
		if err := streamGit(w, work, "fetch", "--progress", "origin"); err != nil {
			return "", fmt.Errorf("git fetch: %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Join(s.root, "scm-work"), 0o755); err != nil {
			return "", err
		}
		fmt.Fprintf(w, "$ git clone --progress %s\n", url)
		if err := streamGit(w, "", "clone", "--progress", url, work); err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
	}
	if branch != "" {
		fmt.Fprintf(w, "$ git checkout %s\n", branch)
		if err := streamGit(w, work, "checkout", "-B", branch, "origin/"+branch); err != nil {
			return "", fmt.Errorf("git checkout %s: %w", branch, err)
		}
	} else {
		fmt.Fprint(w, "$ git reset --hard origin/HEAD\n")
		if err := streamGit(w, work, "reset", "--hard", "origin/HEAD"); err != nil {
			return "", fmt.Errorf("git pull: %w", err)
		}
	}
	return work, nil
}

func (s *server) listBuilds(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	q := `SELECT id, service, version, bin_path, status, branch, git_commit, created_at FROM scm_build`
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
	writeJSON(w, scanRows(rows, "id", "service", "version", "binPath", "status", "branch", "commit", "createdAt"))
}

func (s *server) getBuild(w http.ResponseWriter, r *http.Request) {
	row, err := s.loadBuild(r.PathValue("id"))
	if err == sql.ErrNoRows {
		fail(w, 404, fmt.Errorf("build not found"))
		return
	}
	if err != nil {
		fail(w, 500, err)
		return
	}
	writeJSON(w, row)
}

func (s *server) loadBuild(id string) (map[string]any, error) {
	var bid int64
	var service, version, binPath, status, logText, branch, commit string
	var created time.Time
	err := s.db.QueryRow(
		`SELECT id, service, version, bin_path, status, log_text, branch, git_commit, created_at FROM scm_build WHERE id=?`,
		id,
	).Scan(&bid, &service, &version, &binPath, &status, &logText, &branch, &commit, &created)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": bid, "service": service, "version": version, "binPath": binPath,
		"status": status, "log": logText, "branch": branch, "commit": commit,
		"createdAt": created.Format(time.RFC3339),
	}, nil
}

func (s *server) createBuild(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Branch string `json:"branch"`
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
	branch := strings.TrimSpace(body.Branch)
	if err := checkBranch(branch); err != nil {
		fail(w, 400, err)
		return
	}
	ver := time.Now().Format("20060102-150405")
	binPath := filepath.Join(s.root, "artifacts", job.Name, ver, job.Name)
	res, err := s.db.Exec(
		`INSERT INTO scm_build (service, version, bin_path, status, log_text, branch, git_commit) VALUES (?,?,?,?,?,?,?)`,
		job.Name, ver, binPath, "running", "", branch, "",
	)
	if err != nil {
		fail(w, 500, err)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		fail(w, 500, err)
		return
	}
	live := &liveRun{}
	lives.Store(id, live)
	go s.runBuild(id, job, branch, ver, binPath, live)
	writeJSON(w, map[string]any{
		"id": id, "service": job.Name, "version": ver, "binPath": binPath,
		"status": "running", "branch": branch, "commit": "",
	})
}

func (s *server) streamBuild(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, 400, fmt.Errorf("bad id"))
		return
	}
	lg, err := startSSE(w)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if v, ok := lives.Load(id); ok {
		live := v.(*liveRun)
		snap, ch, cancel := live.follow()
		defer cancel()
		if snap != "" {
			_, _ = lg.Write([]byte(snap))
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case msg, ok := <-ch:
				if !ok {
					live.mu.Lock()
					res := live.result
					live.mu.Unlock()
					if res == nil {
						res = map[string]any{"status": "fail", "error": "stream ended"}
					}
					lg.done(res)
					return
				}
				_, _ = lg.Write([]byte(msg))
			}
		}
	}
	row, err := s.loadBuild(r.PathValue("id"))
	if err != nil {
		lg.done(map[string]any{"status": "fail", "error": err.Error()})
		return
	}
	if logText, _ := row["log"].(string); logText != "" {
		_, _ = lg.Write([]byte(logText))
	}
	lg.done(row)
}

func (s *server) runBuild(id int64, job scmJob, branch, ver, binPath string, live *liveRun) {
	defer lives.Delete(id)
	save := func(status, commit string, runErr error) {
		if runErr != nil {
			fmt.Fprintf(live, "%s\n", runErr)
		}
		logText := live.String()
		_, _ = s.db.Exec(
			`UPDATE scm_build SET bin_path=?, status=?, log_text=?, git_commit=? WHERE id=?`,
			binPath, status, logText, commit, id,
		)
		out := map[string]any{
			"status": status, "service": job.Name, "version": ver,
			"binPath": binPath, "branch": branch, "commit": commit, "id": id,
		}
		if runErr != nil {
			out["error"] = runErr.Error()
		}
		live.finish(out)
	}
	repo, err := s.gitSync(job.Name, job.GitURL, branch, live)
	if err != nil {
		save("fail", "", err)
		return
	}
	commit := ""
	if out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
		fmt.Fprintf(live, "commit %s\n", commit)
	}
	if _, err := scriptInRepo(repo, job.ScriptPath); err != nil {
		save("fail", commit, err)
		return
	}
	outDir := filepath.Dir(binPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		save("fail", commit, err)
		return
	}
	arch := dockerArch()
	fmt.Fprintf(live, "$ sh %s\n(dir=%s out=%s)\n", job.ScriptPath, repo, outDir)
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
	cmd.Stdout = live
	cmd.Stderr = live
	err = cmd.Run()
	status := "ok"
	if err != nil {
		status = "fail"
	} else if _, stErr := os.Stat(binPath); stErr != nil {
		status = "fail"
		err = fmt.Errorf("script ok but missing %s; write the binary to $SCM_OUT/$SCM_NAME", binPath)
	}
	save(status, commit, err)
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
		body.Name, string(raw), string(routes), status, out+"\n"+errStr(err),
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

type deployApp struct {
	ID      int64
	Name    string
	ScmName string
	Compose []string
}

func parseCompose(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func jsonApp(a deployApp, created string) map[string]any {
	return map[string]any{
		"id": a.ID, "name": a.Name, "scmName": a.ScmName,
		"compose": a.Compose, "createdAt": created,
	}
}

func (s *server) listApps(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`SELECT id, name, scm_name, compose, created_at FROM deploy_app ORDER BY id DESC`)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	apps := []map[string]any{}
	for rows.Next() {
		var a deployApp
		var compose, created string
		var t time.Time
		if err := rows.Scan(&a.ID, &a.Name, &a.ScmName, &compose, &t); err != nil {
			continue
		}
		a.Compose = parseCompose(compose)
		created = t.Format(time.RFC3339)
		apps = append(apps, jsonApp(a, created))
	}
	writeJSON(w, map[string]any{"apps": apps})
}

func (s *server) createApp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		ScmName string `json:"scmName"`
		Compose string `json:"compose"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if !jobName.MatchString(name) {
		fail(w, 400, fmt.Errorf("bad deploy name"))
		return
	}
	if _, err := s.getJob(strings.TrimSpace(body.ScmName)); err != nil {
		fail(w, 400, fmt.Errorf("scm: %w", err))
		return
	}
	compose := parseCompose(body.Compose)
	if len(compose) == 0 {
		if c, ok := lookup(name); ok {
			compose = c.Compose
		} else {
			compose = []string{name}
		}
	}
	for _, c := range compose {
		if !jobName.MatchString(c) {
			fail(w, 400, fmt.Errorf("bad compose service %s", c))
			return
		}
	}
	_, err := s.db.Exec(
		`INSERT INTO deploy_app (name, scm_name, compose) VALUES (?,?,?)`,
		name, strings.TrimSpace(body.ScmName), strings.Join(compose, ","),
	)
	if err != nil {
		fail(w, 400, fmt.Errorf("create deploy: %w", err))
		return
	}
	writeJSON(w, map[string]any{"name": name, "scmName": strings.TrimSpace(body.ScmName), "compose": compose})
}

func (s *server) showApp(w http.ResponseWriter, r *http.Request) {
	a, err := s.getApp(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	writeJSON(w, jsonApp(a, ""))
}

func (s *server) getApp(name string) (deployApp, error) {
	var a deployApp
	var compose string
	err := s.db.QueryRow(
		`SELECT id, name, scm_name, compose FROM deploy_app WHERE name=?`, name,
	).Scan(&a.ID, &a.Name, &a.ScmName, &compose)
	if err == sql.ErrNoRows {
		return a, fmt.Errorf("unknown deploy %s", name)
	}
	if err != nil {
		return a, err
	}
	a.Compose = parseCompose(compose)
	return a, nil
}

func (s *server) listDeploys(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	q := `SELECT id, service, version, status, created_at FROM deploy_record`
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
	writeJSON(w, scanRows(rows, "id", "service", "version", "status", "createdAt"))
}

func (s *server) getDeploy(w http.ResponseWriter, r *http.Request) {
	var id int64
	var service, version, status, logText string
	var created time.Time
	err := s.db.QueryRow(
		`SELECT id, service, version, status, log_text, created_at FROM deploy_record WHERE id=?`,
		r.PathValue("id"),
	).Scan(&id, &service, &version, &status, &logText, &created)
	if err == sql.ErrNoRows {
		fail(w, 404, fmt.Errorf("deploy not found"))
		return
	}
	if err != nil {
		fail(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{
		"id": id, "service": service, "version": version,
		"status": status, "log": logText, "createdAt": created.Format(time.RFC3339),
	})
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
	app, err := s.getApp(body.Service)
	if err != nil {
		fail(w, 400, err)
		return
	}
	ver := strings.TrimSpace(body.Version)
	if ver == "" || strings.Contains(ver, "/") || strings.Contains(ver, "..") {
		fail(w, 400, fmt.Errorf("bad version"))
		return
	}
	var st string
	err = s.db.QueryRow(
		`SELECT status FROM scm_build WHERE service=? AND version=? ORDER BY id DESC LIMIT 1`,
		app.ScmName, ver,
	).Scan(&st)
	if err == sql.ErrNoRows {
		fail(w, 400, fmt.Errorf("no scm artifact %s@%s", app.ScmName, ver))
		return
	}
	if err != nil {
		fail(w, 500, err)
		return
	}
	if st != "ok" {
		fail(w, 400, fmt.Errorf("scm %s@%s status %s", app.ScmName, ver, st))
		return
	}
	src := filepath.Join(s.root, "artifacts", app.ScmName, ver, app.ScmName)
	if _, err := os.Stat(src); err != nil {
		fail(w, 400, fmt.Errorf("artifact not found: %s", src))
		return
	}

	lg, err := startSSE(w)
	if err != nil {
		fail(w, 500, err)
		return
	}
	imageVer := fmt.Sprintf("minikitex-%s:%s", app.Name, ver)
	imageLocal := fmt.Sprintf("minikitex-%s:local", app.Name)
	finish := func(status string, runErr error) {
		logText := lg.String()
		if runErr != nil {
			logText += "\n" + runErr.Error()
		}
		if _, dbErr := s.db.Exec(
			`INSERT INTO deploy_record (service, version, status, log_text) VALUES (?,?,?,?)`,
			app.Name, ver, status, logText,
		); dbErr != nil {
			fmt.Fprintf(lg, "save deploy: %s\n", dbErr)
			if runErr == nil {
				runErr = dbErr
				status = "fail"
			}
		}
		out := map[string]any{"status": status, "service": app.Name, "version": ver, "image": imageVer}
		if runErr != nil {
			out["error"] = runErr.Error()
		}
		lg.done(out)
	}

	fmt.Fprintf(lg, "$ docker build -t %s -t %s\n(context %s)\n", imageVer, imageLocal, filepath.Dir(src))
	cmd := exec.Command("docker", "build", "--progress=plain", "-t", imageVer, "-t", imageLocal, "-f", "-", filepath.Dir(src))
	cmd.Stdin = strings.NewReader(fmt.Sprintf("FROM scratch\nCOPY %s /app\nENTRYPOINT [\"/app\"]\n", filepath.Base(src)))
	cmd.Stdout = lg
	cmd.Stderr = lg
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(lg, "%s\n", err)
		finish("fail", err)
		return
	}

	upArgs := append([]string{"--progress=plain", "up", "--no-deps", "--force-recreate", "-d"}, app.Compose...)
	fmt.Fprintf(lg, "$ docker compose up --no-deps --force-recreate -d %s\n", strings.Join(app.Compose, " "))
	if err := s.composeTo(lg, upArgs...); err != nil {
		fmt.Fprintf(lg, "%s\n", err)
		finish("fail", err)
		return
	}
	finish("ok", nil)
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

func (s *server) composeTo(w io.Writer, args ...string) error {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = s.root
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
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

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
