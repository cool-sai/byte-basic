package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	v1 "minikitex/gen/platform/v1"
)

var jobName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type scmJob struct {
	ID         int64
	Name       string
	GitURL     string
	ScriptPath string
	Branch     string
	Label      string
}

func jobProto(j scmJob, created string) *v1.Job {
	return &v1.Job{
		Id: j.ID, Name: j.Name, GitUrl: j.GitURL,
		ScriptPath: j.ScriptPath, Branch: j.Branch, Label: j.Label,
		CreatedAt: created,
	}
}

func checkLabel(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("label required")
	}
	if !jobName.MatchString(s) {
		return fmt.Errorf("bad label")
	}
	return nil
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

func (s *server) ListJobs(context.Context, *connect.Request[v1.ListJobsRequest]) (*connect.Response[v1.ListJobsResponse], error) {
	rows, err := s.db.Query(`SELECT id, name, repo_dir, script_path, branch, label, created_at FROM scm_job ORDER BY id DESC`)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var jobs []*v1.Job
	for rows.Next() {
		var j scmJob
		var t time.Time
		if err := rows.Scan(&j.ID, &j.Name, &j.GitURL, &j.ScriptPath, &j.Branch, &j.Label, &t); err != nil {
			return nil, internal(err)
		}
		jobs = append(jobs, jobProto(j, fmtTime(t)))
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListJobsResponse{Jobs: jobs}), nil
}

func (s *server) CreateJob(_ context.Context, req *connect.Request[v1.CreateJobRequest]) (*connect.Response[v1.Job], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if !jobName.MatchString(name) {
		return nil, invalid(fmt.Errorf("bad scm name"))
	}
	gitURL := strings.TrimSpace(req.Msg.GitUrl)
	if gitURL == "" {
		return nil, invalid(fmt.Errorf("git url required"))
	}
	scriptPath := strings.TrimSpace(req.Msg.ScriptPath)
	if err := checkScriptRel(scriptPath); err != nil {
		return nil, invalid(err)
	}
	label := strings.TrimSpace(req.Msg.Label)
	if err := checkLabel(label); err != nil {
		return nil, invalid(err)
	}
	out, err := exec.Command("git", "ls-remote", "--exit-code", gitURL, "HEAD").CombinedOutput()
	if err != nil {
		return nil, invalid(fmt.Errorf("git ls-remote: %w\n%s", err, out))
	}
	res, err := s.db.Exec(`INSERT INTO scm_job (name, repo_dir, script_path, label) VALUES (?,?,?,?)`, name, gitURL, scriptPath, label)
	if err != nil {
		return nil, invalid(fmt.Errorf("create scm: %w", err))
	}
	id, _ := res.LastInsertId()
	return connect.NewResponse(&v1.Job{Id: id, Name: name, GitUrl: gitURL, ScriptPath: scriptPath, Label: label}), nil
}

func (s *server) ShowJob(_ context.Context, req *connect.Request[v1.ShowJobRequest]) (*connect.Response[v1.Job], error) {
	j, err := s.getJob(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	return connect.NewResponse(jobProto(j, "")), nil
}

func (s *server) UpdateJob(_ context.Context, req *connect.Request[v1.UpdateJobRequest]) (*connect.Response[v1.Job], error) {
	j, err := s.getJob(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	gitURL := strings.TrimSpace(req.Msg.GitUrl)
	if gitURL == "" {
		return nil, invalid(fmt.Errorf("git url required"))
	}
	scriptPath := strings.TrimSpace(req.Msg.ScriptPath)
	if err := checkScriptRel(scriptPath); err != nil {
		return nil, invalid(err)
	}
	label := strings.TrimSpace(req.Msg.Label)
	if err := checkLabel(label); err != nil {
		return nil, invalid(err)
	}
	if gitURL != j.GitURL {
		out, err := exec.Command("git", "ls-remote", "--exit-code", gitURL, "HEAD").CombinedOutput()
		if err != nil {
			return nil, invalid(fmt.Errorf("git ls-remote: %w\n%s", err, out))
		}
	}
	_, err = s.db.Exec(
		`UPDATE scm_job SET repo_dir=?, script_path=?, label=? WHERE name=?`,
		gitURL, scriptPath, label, j.Name,
	)
	if err != nil {
		return nil, internal(err)
	}
	j.GitURL = gitURL
	j.ScriptPath = scriptPath
	j.Label = label
	return connect.NewResponse(jobProto(j, "")), nil
}

func (s *server) DeleteJob(_ context.Context, req *connect.Request[v1.DeleteJobRequest]) (*connect.Response[v1.DeleteJobResponse], error) {
	j, err := s.getJob(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	var used string
	err = s.db.QueryRow(`SELECT name FROM deploy_app WHERE scm_name=? LIMIT 1`, j.Name).Scan(&used)
	if err == nil {
		return nil, invalid(fmt.Errorf("still used by deploy %s", used))
	}
	if err != sql.ErrNoRows {
		return nil, internal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM scm_build WHERE service=?`, j.Name); err != nil {
		return nil, internal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM scm_job WHERE name=?`, j.Name); err != nil {
		return nil, internal(err)
	}
	_ = os.RemoveAll(filepath.Join(s.root, "scm-work", j.Name))
	_ = os.RemoveAll(filepath.Join(s.root, "artifacts", j.Name))
	return connect.NewResponse(&v1.DeleteJobResponse{Name: j.Name}), nil
}

func (s *server) getJob(name string) (scmJob, error) {
	var j scmJob
	err := s.db.QueryRow(
		`SELECT id, name, repo_dir, script_path, branch, label FROM scm_job WHERE name=?`, name,
	).Scan(&j.ID, &j.Name, &j.GitURL, &j.ScriptPath, &j.Branch, &j.Label)
	if err == sql.ErrNoRows {
		return j, fmt.Errorf("unknown scm %s", name)
	}
	return j, err
}

func (s *server) ListBranches(_ context.Context, req *connect.Request[v1.ListBranchesRequest]) (*connect.Response[v1.ListBranchesResponse], error) {
	job, err := s.getJob(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	cmd := exec.Command("git", "ls-remote", "--symref", job.GitURL, "HEAD", "refs/heads/*")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, invalid(fmt.Errorf("git ls-remote: %w\n%s", err, out))
	}
	resp := &v1.ListBranchesResponse{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "ref: ") {
			rest := strings.TrimPrefix(line, "ref: ")
			ref, _, _ := strings.Cut(rest, "\t")
			resp.DefaultBranch = strings.TrimPrefix(ref, "refs/heads/")
			continue
		}
		hash, ref, ok := strings.Cut(line, "\t")
		if !ok || !strings.HasPrefix(ref, "refs/heads/") {
			continue
		}
		resp.Branches = append(resp.Branches, &v1.GitBranch{
			Name:   strings.TrimPrefix(ref, "refs/heads/"),
			Commit: hash,
		})
	}
	return connect.NewResponse(resp), nil
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

var lives sync.Map

type liveRun struct {
	mu     sync.Mutex
	buf    strings.Builder
	cur    strings.Builder
	sent   int
	subs   []chan string
	closed bool
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
	l.mu.Lock()
	defer l.mu.Unlock()
	l.absorb(string(p))
	full := l.buf.String() + l.cur.String()
	if len(full) > l.sent {
		delta := full[l.sent:]
		l.sent = len(full)
		for _, ch := range l.subs {
			select {
			case ch <- delta:
			default:
			}
		}
	}
	return len(p), nil
}

func (l *liveRun) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String() + l.cur.String()
}

func (l *liveRun) subscribe() (string, <-chan string, func()) {
	ch := make(chan string, 64)
	l.mu.Lock()
	defer l.mu.Unlock()
	snap := l.buf.String() + l.cur.String()
	if l.closed {
		close(ch)
		return snap, ch, func() {}
	}
	l.subs = append(l.subs, ch)
	return snap, ch, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		for i, s := range l.subs {
			if s == ch {
				l.subs = append(l.subs[:i], l.subs[i+1:]...)
				break
			}
		}
	}
}

func (l *liveRun) finish(map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	for _, ch := range l.subs {
		close(ch)
	}
	l.subs = nil
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

func (s *server) ListBuilds(_ context.Context, req *connect.Request[v1.ListBuildsRequest]) (*connect.Response[v1.ListBuildsResponse], error) {
	q := `SELECT id, service, version, bin_path, status, branch, git_commit, created_at FROM scm_build`
	var args []any
	if svc := strings.TrimSpace(req.Msg.Service); svc != "" {
		q += ` WHERE service=?`
		args = append(args, svc)
	}
	q += ` ORDER BY id DESC LIMIT 50`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var builds []*v1.Build
	for rows.Next() {
		b := &v1.Build{}
		var t time.Time
		if err := rows.Scan(&b.Id, &b.Service, &b.Version, &b.BinPath, &b.Status, &b.Branch, &b.Commit, &t); err != nil {
			return nil, internal(err)
		}
		b.CreatedAt = fmtTime(t)
		builds = append(builds, b)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListBuildsResponse{Builds: builds}), nil
}

func (s *server) GetBuild(_ context.Context, req *connect.Request[v1.GetBuildRequest]) (*connect.Response[v1.Build], error) {
	b, err := s.loadBuild(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(b), nil
}

func (s *server) WatchBuild(ctx context.Context, req *connect.Request[v1.WatchBuildRequest], stream *connect.ServerStream[v1.RunEvent]) error {
	return s.watchLive(ctx, req.Msg.Id, stream, s.loadBuild)
}

func (s *server) loadBuild(id int64) (*v1.Build, error) {
	b := &v1.Build{}
	var t time.Time
	err := s.db.QueryRow(
		`SELECT id, service, version, bin_path, status, log_text, branch, git_commit, created_at FROM scm_build WHERE id=?`,
		id,
	).Scan(&b.Id, &b.Service, &b.Version, &b.BinPath, &b.Status, &b.Log, &b.Branch, &b.Commit, &t)
	if err == sql.ErrNoRows {
		return nil, notFound(fmt.Errorf("build not found"))
	}
	if err != nil {
		return nil, internal(err)
	}
	b.CreatedAt = fmtTime(t)
	if v, ok := lives.Load(b.Id); ok {
		b.Log = v.(*liveRun).String()
		if b.Status == "running" {
			b.Status = "running"
		}
	}
	if b.Status == "fail" && b.Error == "" {
		b.Error = "fail"
	}
	return b, nil
}

func (s *server) CreateBuild(_ context.Context, req *connect.Request[v1.CreateBuildRequest]) (*connect.Response[v1.Build], error) {
	job, err := s.getJob(req.Msg.Name)
	if err != nil {
		return nil, invalid(err)
	}
	branch := strings.TrimSpace(req.Msg.Branch)
	if err := checkBranch(branch); err != nil {
		return nil, invalid(err)
	}
	ver := time.Now().Format("20060102-150405")
	binPath := filepath.Join(s.root, "artifacts", job.Name, ver, job.Name)
	res, err := s.db.Exec(
		`INSERT INTO scm_build (service, version, bin_path, status, log_text, branch, git_commit) VALUES (?,?,?,?,?,?,?)`,
		job.Name, ver, binPath, "running", "", branch, "",
	)
	if err != nil {
		return nil, internal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, internal(err)
	}
	live := &liveRun{}
	lives.Store(id, live)
	go s.runBuild(id, job, branch, ver, binPath, live)
	return connect.NewResponse(&v1.Build{
		Id: id, Service: job.Name, Version: ver, BinPath: binPath,
		Status: "running", Branch: branch,
	}), nil
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

func (s *server) ListPublishes(context.Context, *connect.Request[v1.ListPublishesRequest]) (*connect.Response[v1.ListPublishesResponse], error) {
	rows, err := s.db.Query(`SELECT id, idl_name, routes_json, status, log_text, created_at FROM agw_publish ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var out []*v1.Publish
	for rows.Next() {
		p := &v1.Publish{}
		var t time.Time
		if err := rows.Scan(&p.Id, &p.IdlName, &p.RoutesJson, &p.Status, &p.Log, &t); err != nil {
			return nil, internal(err)
		}
		p.CreatedAt = fmtTime(t)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListPublishesResponse{Publishes: out}), nil
}

func (s *server) PublishAgw(_ context.Context, req *connect.Request[v1.PublishAgwRequest]) (*connect.Response[v1.PublishAgwResponse], error) {
	path, err := s.idlPath(req.Msg.Name)
	if err != nil {
		return nil, invalid(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, notFound(err)
	}
	view := idlView(req.Msg.Name, string(raw))
	if view.ParseError != "" {
		return nil, invalid(fmt.Errorf("%s", view.ParseError))
	}
	routes, _ := json.Marshal(view.Methods)
	out, err := s.compose("restart", "gateway")
	status := "ok"
	if err != nil {
		status = "fail"
	}
	_, dbErr := s.db.Exec(
		`INSERT INTO agw_publish (idl_name, content, routes_json, status, log_text) VALUES (?,?,?,?,?)`,
		req.Msg.Name, string(raw), string(routes), status, out+"\n"+errStr(err),
	)
	if dbErr != nil {
		return nil, internal(dbErr)
	}
	if err != nil {
		return nil, internal(fmt.Errorf("agw publish: %w\n%s", err, out))
	}
	return connect.NewResponse(&v1.PublishAgwResponse{Name: req.Msg.Name, Status: status, Methods: view.Methods}), nil
}

type deployApp struct {
	ID      int64
	Name    string
	ScmName string
	Compose []string
	Label   string
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

func appProto(a deployApp, created string) *v1.App {
	return &v1.App{
		Id: a.ID, Name: a.Name, ScmName: a.ScmName,
		Compose: a.Compose, Label: a.Label, CreatedAt: created,
	}
}

func (s *server) ListApps(context.Context, *connect.Request[v1.ListAppsRequest]) (*connect.Response[v1.ListAppsResponse], error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.name, a.scm_name, a.compose, a.created_at, IFNULL(j.label,'')
		FROM deploy_app a
		LEFT JOIN scm_job j ON j.name = a.scm_name
		ORDER BY a.id DESC`)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var apps []*v1.App
	for rows.Next() {
		var a deployApp
		var compose string
		var t time.Time
		if err := rows.Scan(&a.ID, &a.Name, &a.ScmName, &compose, &t, &a.Label); err != nil {
			continue
		}
		a.Compose = parseCompose(compose)
		apps = append(apps, appProto(a, fmtTime(t)))
	}
	return connect.NewResponse(&v1.ListAppsResponse{Apps: apps}), nil
}

func (s *server) CreateApp(_ context.Context, req *connect.Request[v1.CreateAppRequest]) (*connect.Response[v1.App], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if !jobName.MatchString(name) {
		return nil, invalid(fmt.Errorf("bad deploy name"))
	}
	scmName := strings.TrimSpace(req.Msg.ScmName)
	if _, err := s.getJob(scmName); err != nil {
		return nil, invalid(fmt.Errorf("scm: %w", err))
	}
	compose := parseCompose(req.Msg.Compose)
	if len(compose) == 0 {
		if c, ok := lookup(name); ok {
			compose = c.Compose
		} else {
			compose = []string{name}
		}
	}
	for _, svc := range compose {
		if !jobName.MatchString(svc) {
			return nil, invalid(fmt.Errorf("bad compose service %s", svc))
		}
	}
	res, err := s.db.Exec(
		`INSERT INTO deploy_app (name, scm_name, compose) VALUES (?,?,?)`,
		name, scmName, strings.Join(compose, ","),
	)
	if err != nil {
		return nil, invalid(fmt.Errorf("create deploy: %w", err))
	}
	id, _ := res.LastInsertId()
	return connect.NewResponse(&v1.App{Id: id, Name: name, ScmName: scmName, Compose: compose}), nil
}

func (s *server) ShowApp(_ context.Context, req *connect.Request[v1.ShowAppRequest]) (*connect.Response[v1.App], error) {
	a, err := s.getApp(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	return connect.NewResponse(appProto(a, "")), nil
}

func (s *server) getApp(name string) (deployApp, error) {
	var a deployApp
	var compose string
	err := s.db.QueryRow(
		`SELECT a.id, a.name, a.scm_name, a.compose, IFNULL(j.label,'')
		 FROM deploy_app a LEFT JOIN scm_job j ON j.name = a.scm_name WHERE a.name=?`, name,
	).Scan(&a.ID, &a.Name, &a.ScmName, &compose, &a.Label)
	if err == sql.ErrNoRows {
		return a, fmt.Errorf("unknown deploy %s", name)
	}
	if err != nil {
		return a, err
	}
	a.Compose = parseCompose(compose)
	return a, nil
}

func (s *server) ListDeploys(_ context.Context, req *connect.Request[v1.ListDeploysRequest]) (*connect.Response[v1.ListDeploysResponse], error) {
	q := `SELECT id, service, version, status, created_at FROM deploy_record`
	var args []any
	if svc := strings.TrimSpace(req.Msg.Service); svc != "" {
		q += ` WHERE service=?`
		args = append(args, svc)
	}
	q += ` ORDER BY id DESC LIMIT 50`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var out []*v1.Deploy
	for rows.Next() {
		d := &v1.Deploy{}
		var t time.Time
		if err := rows.Scan(&d.Id, &d.Service, &d.Version, &d.Status, &t); err != nil {
			return nil, internal(err)
		}
		d.CreatedAt = fmtTime(t)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListDeploysResponse{Deploys: out}), nil
}

func (s *server) GetDeploy(_ context.Context, req *connect.Request[v1.GetDeployRequest]) (*connect.Response[v1.Deploy], error) {
	d, err := s.loadDeploy(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(d), nil
}

func (s *server) WatchDeploy(ctx context.Context, req *connect.Request[v1.WatchDeployRequest], stream *connect.ServerStream[v1.RunEvent]) error {
	return s.watchLive(ctx, req.Msg.Id, stream, func(id int64) (*v1.Build, error) {
		d, err := s.loadDeploy(id)
		if err != nil {
			return nil, err
		}
		return &v1.Build{Id: d.Id, Service: d.Service, Version: d.Version, Status: d.Status, Log: d.Log, CreatedAt: d.CreatedAt, Error: d.Error}, nil
	})
}

func (s *server) loadDeploy(id int64) (*v1.Deploy, error) {
	d := &v1.Deploy{}
	var t time.Time
	err := s.db.QueryRow(
		`SELECT id, service, version, status, log_text, created_at FROM deploy_record WHERE id=?`,
		id,
	).Scan(&d.Id, &d.Service, &d.Version, &d.Status, &d.Log, &t)
	if err == sql.ErrNoRows {
		return nil, notFound(fmt.Errorf("deploy not found"))
	}
	if err != nil {
		return nil, internal(err)
	}
	d.CreatedAt = fmtTime(t)
	if v, ok := lives.Load(d.Id); ok {
		d.Log = v.(*liveRun).String()
	}
	if d.Status == "fail" && d.Error == "" {
		d.Error = "fail"
	}
	return d, nil
}

func (s *server) CreateDeploy(_ context.Context, req *connect.Request[v1.CreateDeployRequest]) (*connect.Response[v1.Deploy], error) {
	app, err := s.getApp(req.Msg.Service)
	if err != nil {
		return nil, invalid(err)
	}
	ver := strings.TrimSpace(req.Msg.Version)
	if ver == "" || strings.Contains(ver, "/") || strings.Contains(ver, "..") {
		return nil, invalid(fmt.Errorf("bad version"))
	}
	var st string
	err = s.db.QueryRow(
		`SELECT status FROM scm_build WHERE service=? AND version=? ORDER BY id DESC LIMIT 1`,
		app.ScmName, ver,
	).Scan(&st)
	if err == sql.ErrNoRows {
		return nil, invalid(fmt.Errorf("no scm artifact %s@%s", app.ScmName, ver))
	}
	if err != nil {
		return nil, internal(err)
	}
	if st != "ok" {
		return nil, invalid(fmt.Errorf("scm %s@%s status %s", app.ScmName, ver, st))
	}
	src := filepath.Join(s.root, "artifacts", app.ScmName, ver, app.ScmName)
	if _, err := os.Stat(src); err != nil {
		return nil, invalid(fmt.Errorf("artifact not found: %s", src))
	}
	job, err := s.getJob(app.ScmName)
	if err != nil {
		return nil, invalid(err)
	}
	label := job.Label
	if label == "" {
		label = "golang"
	}
	res, err := s.db.Exec(
		`INSERT INTO deploy_record (service, version, status, log_text) VALUES (?,?,?,?)`,
		app.Name, ver, "running", "",
	)
	if err != nil {
		return nil, internal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, internal(err)
	}
	live := &liveRun{}
	lives.Store(id, live)
	go s.runDeploy(id, app, ver, src, label, live)
	return connect.NewResponse(&v1.Deploy{Id: id, Service: app.Name, Version: ver, Status: "running"}), nil
}

func (s *server) runDeploy(id int64, app deployApp, ver, src, label string, live *liveRun) {
	defer lives.Delete(id)
	imageVer := fmt.Sprintf("minikitex-%s:%s", app.Name, ver)
	imageLocal := fmt.Sprintf("minikitex-%s:local", app.Name)
	finish := func(status string, runErr error) {
		if runErr != nil {
			fmt.Fprintf(live, "%s\n", runErr)
		}
		_, _ = s.db.Exec(`UPDATE deploy_record SET status=?, log_text=? WHERE id=?`, status, live.String(), id)
		out := map[string]any{"status": status, "service": app.Name, "version": ver, "image": imageVer, "id": id}
		if runErr != nil {
			out["error"] = runErr.Error()
		}
		live.finish(out)
	}
	fmt.Fprintf(live, "$ docker build -t %s -t %s  label=%s\n", imageVer, imageLocal, label)
	if label == "node" {
		if err := s.buildNodeImage(live, src, imageVer, imageLocal); err != nil {
			finish("fail", err)
			return
		}
	} else if label == "python" {
		if err := s.buildPythonImage(live, src, imageVer, imageLocal); err != nil {
			finish("fail", err)
			return
		}
	} else {
		fmt.Fprintf(live, "(context %s)\n", filepath.Dir(src))
		cmd := exec.Command("docker", "build", "-t", imageVer, "-t", imageLocal, "-f", "-", filepath.Dir(src))
		cmd.Stdin = strings.NewReader(fmt.Sprintf("FROM scratch\nCOPY %s /app\nENTRYPOINT [\"/app\"]\n", filepath.Base(src)))
		cmd.Stdout = live
		cmd.Stderr = live
		if err := cmd.Run(); err != nil {
			finish("fail", err)
			return
		}
	}
	if err := s.syncRuntimeCompose(); err != nil {
		finish("fail", err)
		return
	}
	upArgs := append([]string{"up", "--no-deps", "--force-recreate", "-d"}, app.Compose...)
	fmt.Fprintf(live, "$ docker compose up --no-deps --force-recreate -d %s\n", strings.Join(app.Compose, " "))
	if err := s.composeTo(live, upArgs...); err != nil {
		finish("fail", err)
		return
	}
	finish("ok", nil)
}

func (s *server) Runtime(context.Context, *connect.Request[v1.RuntimeRequest]) (*connect.Response[v1.RuntimeResponse], error) {
	out, err := s.compose("ps", "--format", "json")
	if err != nil {
		return nil, internal(fmt.Errorf("%w\n%s", err, out))
	}
	dec := json.NewDecoder(strings.NewReader(out))
	var items []*v1.Container
	for {
		var one map[string]any
		if err := dec.Decode(&one); err != nil {
			if err == io.EOF {
				break
			}
			return nil, internal(err)
		}
		str := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := one[k]; ok && v != nil {
					return fmt.Sprint(v)
				}
			}
			return ""
		}
		items = append(items, &v1.Container{
			Id:      str("ID", "Id"),
			Name:    str("Name"),
			Service: str("Service"),
			Image:   str("Image"),
			Status:  str("Status"),
			State:   str("State"),
		})
	}
	return connect.NewResponse(&v1.RuntimeResponse{Containers: items}), nil
}

func (s *server) composeCmd(args ...string) *exec.Cmd {
	prefix := []string{"compose"}
	rt := filepath.Join(s.root, "deploy", "runtime.yml")
	if _, err := os.Stat(rt); err == nil {
		prefix = append(prefix, "-f", "docker-compose.yml", "-f", "deploy/runtime.yml")
	}
	cmd := exec.Command("docker", append(prefix, args...)...)
	cmd.Dir = s.root
	return cmd
}

func (s *server) compose(args ...string) (string, error) {
	b, err := s.composeCmd(args...).CombinedOutput()
	return string(b), err
}

func (s *server) composeTo(w io.Writer, args ...string) error {
	cmd := s.composeCmd(args...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

const nodeWebDockerfile = `FROM alpine:3.20
RUN apk add --no-cache nginx && mkdir -p /run/nginx
COPY html /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`

const nodeWebNginx = `worker_processes auto;
error_log /dev/stderr warn;
pid /run/nginx/nginx.pid;
events {
    worker_connections 1024;
}
http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    sendfile on;
    server {
        listen 80;
        root /usr/share/nginx/html;
        location / {
            try_files $uri $uri/ /index.html;
        }
    }
}
`

func (s *server) buildNodeImage(lg io.Writer, src, imageVer, imageLocal string) error {
	tmp, err := os.MkdirTemp("", "mk-node-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	html := filepath.Join(tmp, "html")
	if err := os.MkdirAll(html, 0o755); err != nil {
		return err
	}
	fmt.Fprintf(lg, "unpack %s\n", src)
	untar := exec.Command("tar", "-xf", src, "-C", html)
	untar.Stdout = lg
	untar.Stderr = lg
	if err := untar.Run(); err != nil {
		return fmt.Errorf("unpack node artifact: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "nginx.conf"), []byte(nodeWebNginx), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(nodeWebDockerfile), 0o644); err != nil {
		return err
	}
	build := exec.Command("docker", "build", "-t", imageVer, "-t", imageLocal, tmp)
	build.Stdout = lg
	build.Stderr = lg
	return build.Run()
}

const pythonWebDockerfile = `FROM alpine:3.20
RUN apk add --no-cache python3 py3-pip
WORKDIR /app
COPY app /app
RUN pip3 install --no-cache-dir --break-system-packages -r requirements.txt
EXPOSE 80
CMD ["sh", "-c", "gunicorn -b ${LISTEN:-0.0.0.0:80} app:app"]
`

func (s *server) buildPythonImage(lg io.Writer, src, imageVer, imageLocal string) error {
	tmp, err := os.MkdirTemp("", "mk-py-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	appDir := filepath.Join(tmp, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}
	fmt.Fprintf(lg, "unpack %s\n", src)
	untar := exec.Command("tar", "-xf", src, "-C", appDir)
	untar.Stdout = lg
	untar.Stderr = lg
	if err := untar.Run(); err != nil {
		return fmt.Errorf("unpack python artifact: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(pythonWebDockerfile), 0o644); err != nil {
		return err
	}
	build := exec.Command("docker", "build", "-t", imageVer, "-t", imageLocal, tmp)
	build.Stdout = lg
	build.Stderr = lg
	return build.Run()
}

func (s *server) syncRuntimeCompose() error {
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "config", "--services")
	cmd.Dir = s.root
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose config: %w\n%s", err, b)
	}
	main := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			main[line] = true
		}
	}
	rows, err := s.db.Query(`
		SELECT a.name, a.compose, IFNULL(j.label,'')
		FROM deploy_app a LEFT JOIN scm_job j ON j.name = a.scm_name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type extra struct{ svc, image, label string }
	var extras []extra
	seen := map[string]bool{}
	for rows.Next() {
		var name, compose, label string
		if err := rows.Scan(&name, &compose, &label); err != nil {
			return err
		}
		if label == "" {
			label = "golang"
		}
		if label != "node" && label != "golang" {
			if label != "python" {
				continue
			}
		}
		image := "minikitex-" + name + ":local"
		for _, c := range parseCompose(compose) {
			if main[c] || seen[c] {
				continue
			}
			seen[c] = true
			extras = append(extras, extra{svc: c, image: image, label: label})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	path := filepath.Join(s.root, "deploy", "runtime.yml")
	if len(extras) == 0 {
		_ = os.Remove(path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var y strings.Builder
	y.WriteString("services:\n")
	for _, e := range extras {
		fmt.Fprintf(&y, "  %s:\n    image: %s\n    hostname: %s\n    pull_policy: never\n    expose:\n      - \"80\"\n    labels:\n      psm: %s\n", e.svc, e.image, e.svc, e.svc)
		if e.label == "golang" {
			y.WriteString("    environment:\n      LISTEN: \"0.0.0.0:80\"\n")
		} else {
			if e.label == "python" {
				y.WriteString("    environment:\n      LISTEN: \"0.0.0.0:80\"\n")
			}
		}
	}
	return os.WriteFile(path, []byte(y.String()), 0o644)
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

func fmtTime(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02 15:04:05")
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
