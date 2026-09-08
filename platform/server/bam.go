package main

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/bufbuild/protocompile"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	v1 "minikitex/gen/platform/v1"
)

var bamName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._][a-z0-9]+)*$`)
var bamPath = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*\.proto$`)

func (s *server) seedBam() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM bam_module`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	p := filepath.Join(s.root, "proto", "platform", "v1", "platform.proto")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	files := []*v1.BamFile{{Path: "platform/v1/platform.proto", Content: string(b)}}
	_, err = s.writeModule("platform", files, "", "", "", "")
	return err
}

func (s *server) ListBamModules(context.Context, *connect.Request[v1.ListBamModulesRequest]) (*connect.Response[v1.ListBamModulesResponse], error) {
	rows, err := s.db.Query(`SELECT id, name, version, rpcs, http_apis, scm_name, branch, proto_dir, git_commit, created_at FROM bam_module ORDER BY id`)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var out []*v1.BamModule
	for rows.Next() {
		m, err := scanBamModule(rows)
		if err != nil {
			return nil, internal(err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListBamModulesResponse{Modules: out}), nil
}

func (s *server) CreateBamModule(_ context.Context, req *connect.Request[v1.CreateBamModuleRequest]) (*connect.Response[v1.BamModule], error) {
	name := strings.TrimSpace(req.Msg.Name)
	scmName := strings.TrimSpace(req.Msg.ScmName)
	protoDir := strings.Trim(filepath.ToSlash(strings.TrimSpace(req.Msg.ProtoDir)), "/")
	branch := strings.TrimSpace(req.Msg.Branch)
	if err := checkBamName(name); err != nil {
		return nil, invalid(err)
	}
	if scmName == "" {
		return nil, invalid(fmt.Errorf("请选择仓库"))
	}
	if err := checkProtoDir(protoDir); err != nil {
		return nil, invalid(err)
	}
	if branch != "" {
		if err := checkBranch(branch); err != nil {
			return nil, invalid(err)
		}
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM bam_module WHERE name=?`, name).Scan(&n); err != nil {
		return nil, internal(err)
	}
	if n > 0 {
		return nil, invalid(fmt.Errorf("module %s exists", name))
	}
	var taken string
	err := s.db.QueryRow(`SELECT name FROM bam_module WHERE scm_name=? AND proto_dir=?`, scmName, protoDir).Scan(&taken)
	if err == nil {
		return nil, invalid(fmt.Errorf("目录 %s 已绑定为 %s", protoDir, taken))
	}
	if err != sql.ErrNoRows {
		return nil, internal(err)
	}
	repo, err := s.syncBamRepo(scmName, branch, io.Discard)
	if err != nil {
		return nil, invalid(err)
	}
	files, err := collectBamFiles(repo, protoDir)
	if err != nil {
		return nil, invalid(err)
	}
	mod, err := s.writeModule(name, files, scmName, branch, protoDir, gitHead(repo))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(mod), nil
}

func (s *server) GetBamModule(_ context.Context, req *connect.Request[v1.GetBamModuleRequest]) (*connect.Response[v1.BamModuleDetail], error) {
	d, err := s.loadBam(strings.TrimSpace(req.Msg.Name))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(d), nil
}

func (s *server) SaveBamModule(_ context.Context, req *connect.Request[v1.SaveBamModuleRequest]) (*connect.Response[v1.BamModuleDetail], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if err := checkBamName(name); err != nil {
		return nil, invalid(err)
	}
	files, err := checkBamFiles(req.Msg.Files)
	if err != nil {
		return nil, invalid(err)
	}
	var id int64
	var scm string
	if err := s.db.QueryRow(`SELECT id, scm_name FROM bam_module WHERE name=?`, name).Scan(&id, &scm); err != nil {
		if err == sql.ErrNoRows {
			return nil, notFound(fmt.Errorf("no bam module %s", name))
		}
		return nil, internal(err)
	}
	if scm != "" {
		return nil, invalid(fmt.Errorf("请在仓库里改 proto，然后点生成"))
	}
	if _, err := s.writeModule(name, files, "", "", "", ""); err != nil {
		return nil, err
	}
	d, err := s.loadBam(name)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(d), nil
}

func (s *server) GenerateBam(_ context.Context, req *connect.Request[v1.GenerateBamRequest]) (*connect.Response[v1.BamGenerateResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	d, err := s.loadBam(name)
	if err != nil {
		return nil, err
	}
	var repo, gitLog string
	if d.Module.ScmName != "" {
		var buf bytes.Buffer
		repo, err = s.syncBamRepo(d.Module.ScmName, d.Module.Branch, &buf)
		gitLog = buf.String()
		if err != nil {
			return nil, invalid(fmt.Errorf("%s%w", gitLog, err))
		}
		files, err := collectBamFiles(repo, d.Module.ProtoDir)
		if err != nil {
			return nil, invalid(err)
		}
		if _, err := s.writeModule(name, files, d.Module.ScmName, d.Module.Branch, d.Module.ProtoDir, gitHead(repo)); err != nil {
			return nil, err
		}
		d, err = s.loadBam(name)
		if err != nil {
			return nil, err
		}
	}
	if d.ParseError != "" {
		return nil, invalid(fmt.Errorf("%s", d.ParseError))
	}
	out, err := s.runBamGen(d, repo)
	if err != nil {
		return nil, err
	}
	if gitLog != "" {
		out.Log = gitLog + out.Log
	}
	return connect.NewResponse(out), nil
}

func (s *server) DownloadBam(_ context.Context, req *connect.Request[v1.DownloadBamRequest]) (*connect.Response[v1.BamDownloadResponse], error) {
	filename, zipb, err := s.bamZip(strings.TrimSpace(req.Msg.Name), int(req.Msg.Version))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.BamDownloadResponse{Filename: filename, Zip: zipb}), nil
}

func (s *server) bamZip(name string, ver int) (string, []byte, error) {
	mod, err := s.getBamModule(name)
	if err != nil {
		return "", nil, err
	}
	if ver <= 0 {
		ver = int(mod.Version)
	}
	var dir, status string
	err = s.db.QueryRow(
		`SELECT artifact_dir, status FROM bam_gen WHERE module_id=? AND version=?`,
		mod.Id, ver,
	).Scan(&dir, &status)
	if err == sql.ErrNoRows {
		return "", nil, notFound(fmt.Errorf("尚未生成 %s@%d", name, ver))
	}
	if err != nil {
		return "", nil, internal(err)
	}
	if status != "ok" || dir == "" {
		return "", nil, invalid(fmt.Errorf("生成未成功"))
	}
	zipb, err := zipDir(dir)
	if err != nil {
		return "", nil, internal(err)
	}
	return fmt.Sprintf("%s-v%d.zip", name, ver), zipb, nil
}

func bamZipName(path string) (string, bool) {
	const pre = "/api/bam/modules/"
	const suf = "/download.zip"
	if !strings.HasPrefix(path, pre) || !strings.HasSuffix(path, suf) {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(path, pre), suf)
	if name == "" || strings.Contains(name, "/") {
		return "", false
	}
	return name, true
}

func (s *server) handleBamZip(w http.ResponseWriter, r *http.Request) {
	name, ok := bamZipName(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ver := 0
	if q := strings.TrimSpace(r.URL.Query().Get("version")); q != "" {
		ver, _ = strconv.Atoi(q)
	}
	filename, zipb, err := s.bamZip(name, ver)
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write(zipb)
}

func writeHTTPError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	msg := err.Error()
	var ce *connect.Error
	if errors.As(err, &ce) {
		msg = ce.Message()
		switch ce.Code() {
		case connect.CodeNotFound:
			code = http.StatusNotFound
		case connect.CodeInvalidArgument:
			code = http.StatusBadRequest
		case connect.CodeUnauthenticated:
			code = http.StatusUnauthorized
		}
	}
	http.Error(w, msg, code)
}

func (s *server) writeModule(name string, files []*v1.BamFile, scmName, branch, protoDir, commit string) (*v1.BamModule, error) {
	rpcs, httpN, parseErr := compileBam(s.root, files)
	rpcCount := int32(len(rpcs))
	if parseErr != "" {
		rpcCount = 0
		httpN = 0
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, internal(err)
	}
	defer tx.Rollback()
	var id int64
	var ver int
	err = tx.QueryRow(`SELECT id, version FROM bam_module WHERE name=?`, name).Scan(&id, &ver)
	if err == sql.ErrNoRows {
		res, err := tx.Exec(
			`INSERT INTO bam_module (name, version, rpcs, http_apis, scm_name, branch, proto_dir, git_commit) VALUES (?,?,?,?,?,?,?,?)`,
			name, 1, rpcCount, httpN, scmName, branch, protoDir, commit,
		)
		if err != nil {
			return nil, internal(err)
		}
		id, _ = res.LastInsertId()
		ver = 1
	} else if err != nil {
		return nil, internal(err)
	} else {
		ver++
		if scmName != "" {
			_, err = tx.Exec(
				`UPDATE bam_module SET version=?, rpcs=?, http_apis=?, scm_name=?, branch=?, proto_dir=?, git_commit=? WHERE id=?`,
				ver, rpcCount, httpN, scmName, branch, protoDir, commit, id,
			)
		} else {
			_, err = tx.Exec(
				`UPDATE bam_module SET version=?, rpcs=?, http_apis=? WHERE id=?`,
				ver, rpcCount, httpN, id,
			)
		}
		if err != nil {
			return nil, internal(err)
		}
	}
	for _, f := range files {
		if _, err := tx.Exec(
			`INSERT INTO bam_file (module_id, version, path, content) VALUES (?,?,?,?)`,
			id, ver, f.Path, f.Content,
		); err != nil {
			return nil, internal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, internal(err)
	}
	m := &v1.BamModule{
		Id: id, Name: name, Version: int32(ver), Rpcs: rpcCount, HttpApis: httpN,
		ScmName: scmName, Branch: branch, ProtoDir: protoDir, GitCommit: commit,
	}
	return m, nil
}

func (s *server) getBamModule(name string) (*v1.BamModule, error) {
	if err := checkBamName(name); err != nil {
		return nil, invalid(err)
	}
	row := s.db.QueryRow(`SELECT id, name, version, rpcs, http_apis, scm_name, branch, proto_dir, git_commit, created_at FROM bam_module WHERE name=?`, name)
	m, err := scanBamModule(row)
	if err == sql.ErrNoRows {
		return nil, notFound(fmt.Errorf("no bam module %s", name))
	}
	if err != nil {
		return nil, internal(err)
	}
	return m, nil
}

func (s *server) loadBam(name string) (*v1.BamModuleDetail, error) {
	mod, err := s.getBamModule(name)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT path, content FROM bam_file WHERE module_id=? AND version=? ORDER BY path`,
		mod.Id, mod.Version,
	)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var files []*v1.BamFile
	for rows.Next() {
		f := &v1.BamFile{}
		if err := rows.Scan(&f.Path, &f.Content); err != nil {
			return nil, internal(err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	rpcs, httpN, parseErr := compileBam(s.root, files)
	mod.Rpcs = int32(len(rpcs))
	mod.HttpApis = httpN
	if parseErr != "" {
		mod.Rpcs = 0
		mod.HttpApis = 0
	}
	d := &v1.BamModuleDetail{Module: mod, Files: files, Rpcs: rpcs, ParseError: parseErr}
	var st, dir string
	err = s.db.QueryRow(
		`SELECT status, artifact_dir FROM bam_gen WHERE module_id=? AND version=?`,
		mod.Id, mod.Version,
	).Scan(&st, &dir)
	if err == nil {
		d.GenStatus = st
		d.GenDir = dir
	} else if err != sql.ErrNoRows {
		return nil, internal(err)
	}
	return d, nil
}

func (s *server) runBamGen(d *v1.BamModuleDetail, repo string) (*v1.BamGenerateResponse, error) {
	ensurePluginPath()
	if _, err := exec.LookPath("buf"); err != nil {
		return nil, invalid(fmt.Errorf("未找到 buf，请安装后重试"))
	}
	if _, err := exec.LookPath("protoc-gen-go"); err != nil {
		return nil, invalid(fmt.Errorf("未找到 protoc-gen-go"))
	}
	if _, err := exec.LookPath("protoc-gen-connect-go"); err != nil {
		return nil, invalid(fmt.Errorf("未找到 protoc-gen-connect-go"))
	}
	mod := d.Module
	work, err := os.MkdirTemp("", "bam-gen-*")
	if err != nil {
		return nil, internal(err)
	}
	defer os.RemoveAll(work)
	protoDir := filepath.Join(work, "proto")
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		return nil, internal(err)
	}
	pathArg := "proto"
	if repo != "" && mod.ProtoDir != "" {
		rootRel := protoModuleRoot(repo, mod.ProtoDir)
		if err := copyTree(filepath.Join(repo, filepath.FromSlash(rootRel)), protoDir); err != nil {
			return nil, internal(err)
		}
		rel := "."
		if r, err := filepath.Rel(filepath.FromSlash(rootRel), filepath.FromSlash(mod.ProtoDir)); err == nil {
			rel = filepath.ToSlash(r)
		}
		if rel != "." {
			pathArg = filepath.ToSlash(filepath.Join("proto", rel))
		}
	} else {
		for _, f := range d.Files {
			dst := filepath.Join(protoDir, filepath.FromSlash(f.Path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return nil, internal(err)
			}
			if err := os.WriteFile(dst, []byte(f.Content), 0o644); err != nil {
				return nil, internal(err)
			}
		}
	}
	if err := copyGoogleAPI(s.root, protoDir); err != nil {
		return nil, internal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "buf.yaml"), []byte("version: v2\nmodules:\n  - path: proto\nlint:\n  use:\n    - MINIMAL\n"), 0o644); err != nil {
		return nil, internal(err)
	}
	genYAML, err := bamGenYAML(s.root)
	if err != nil {
		return nil, invalid(err)
	}
	if err := os.WriteFile(filepath.Join(work, "buf.gen.yaml"), []byte(genYAML), 0o644); err != nil {
		return nil, internal(err)
	}
	args := []string{"generate", "--path", pathArg}
	if _, err := os.Stat(filepath.Join(work, "proto", "google")); err == nil {
		args = append(args, "--exclude-path", "proto/google")
	}
	cmd := exec.Command("buf", args...)
	cmd.Dir = work
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	logText := string(out)
	status := "ok"
	if err != nil {
		status = "fail"
		logText = strings.TrimSpace(logText + "\n" + err.Error())
	}
	art := filepath.Join(s.root, "artifacts", "bam", mod.Name, fmt.Sprintf("v%d", mod.Version))
	_ = os.RemoveAll(art)
	if status == "ok" {
		if err := os.MkdirAll(art, 0o755); err != nil {
			return nil, internal(err)
		}
		if err := copyTree(filepath.Join(work, "gen"), art); err != nil {
			status = "fail"
			logText = strings.TrimSpace(logText + "\n" + err.Error())
		}
	}
	if _, err := s.db.Exec(
		`INSERT INTO bam_gen (module_id, version, status, log_text, artifact_dir) VALUES (?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE status=VALUES(status), log_text=VALUES(log_text), artifact_dir=VALUES(artifact_dir), created_at=CURRENT_TIMESTAMP`,
		mod.Id, mod.Version, status, logText, art,
	); err != nil {
		return nil, internal(err)
	}
	files, _ := listRel(art)
	if status != "ok" {
		return &v1.BamGenerateResponse{Version: mod.Version, Status: status, Log: logText, Dir: art, Files: files}, nil
	}
	return &v1.BamGenerateResponse{Version: mod.Version, Status: status, Log: logText, Dir: art, Files: files}, nil
}

func compileBam(root string, files []*v1.BamFile) ([]*v1.BamRpc, int32, string) {
	byPath := map[string]string{}
	var names []string
	for _, f := range files {
		byPath[filepath.ToSlash(f.Path)] = f.Content
		names = append(names, filepath.ToSlash(f.Path))
	}
	acc := func(path string) (io.ReadCloser, error) {
		path = filepath.ToSlash(path)
		if c, ok := byPath[path]; ok {
			return io.NopCloser(strings.NewReader(c)), nil
		}
		p := filepath.Join(root, "proto", filepath.FromSlash(path))
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	compiler := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(&protocompile.SourceResolver{Accessor: acc}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	fds, err := compiler.Compile(context.Background(), names...)
	if err != nil {
		return nil, 0, err.Error()
	}
	var rpcs []*v1.BamRpc
	var httpN int32
	for _, fd := range fds {
		if strings.HasPrefix(fd.Path(), "google/") {
			continue
		}
		for si := 0; si < fd.Services().Len(); si++ {
			svc := fd.Services().Get(si)
			for mi := 0; mi < svc.Methods().Len(); mi++ {
				md := svc.Methods().Get(mi)
				item := &v1.BamRpc{
					Service:    string(svc.Name()),
					Name:       string(md.Name()),
					Req:        string(md.Input().Name()),
					Resp:       string(md.Output().Name()),
					Comment:    commentOf(fd, md),
					Stream:     md.IsStreamingServer() || md.IsStreamingClient(),
					ReqFields:  fieldsOfMsg(fd, md.Input()),
					RespFields: fieldsOfMsg(fd, md.Output()),
				}
				item.HttpMethod, item.Uri = httpOf(md)
				if item.Uri != "" {
					httpN++
				}
				rpcs = append(rpcs, item)
			}
		}
	}
	return rpcs, httpN, ""
}

func fieldsOfMsg(fd protoreflect.FileDescriptor, msg protoreflect.MessageDescriptor) []*v1.BamField {
	var out []*v1.BamField
	for i := 0; i < msg.Fields().Len(); i++ {
		f := msg.Fields().Get(i)
		out = append(out, &v1.BamField{
			Id:      int32(f.Number()),
			Type:    fieldType(f),
			Name:    string(f.Name()),
			Comment: commentOf(fd, f),
		})
	}
	return out
}

func fieldType(f protoreflect.FieldDescriptor) string {
	if f.IsMap() {
		return "map<" + kindName(f.MapKey()) + "," + kindName(f.MapValue()) + ">"
	}
	t := kindName(f)
	if f.Cardinality() == protoreflect.Repeated {
		return "repeated " + t
	}
	return t
}

func kindName(f protoreflect.FieldDescriptor) string {
	if f.Kind() == protoreflect.MessageKind && f.Message() != nil {
		return string(f.Message().Name())
	}
	if f.Kind() == protoreflect.EnumKind && f.Enum() != nil {
		return string(f.Enum().Name())
	}
	return f.Kind().String()
}

func commentOf(fd protoreflect.FileDescriptor, d protoreflect.Descriptor) string {
	loc := fd.SourceLocations().ByDescriptor(d)
	c := strings.TrimSpace(loc.LeadingComments)
	c = strings.ReplaceAll(c, "\n", " ")
	return strings.TrimSpace(c)
}

func httpOf(md protoreflect.MethodDescriptor) (string, string) {
	opts := md.Options()
	if opts == nil {
		return "", ""
	}
	raw, err := proto.Marshal(opts)
	if err != nil {
		return "", ""
	}
	mo := &descriptorpb.MethodOptions{}
	if err := proto.Unmarshal(raw, mo); err != nil {
		return "", ""
	}
	if !proto.HasExtension(mo, annotations.E_Http) {
		return "", ""
	}
	rule, ok := proto.GetExtension(mo, annotations.E_Http).(*annotations.HttpRule)
	if !ok || rule == nil {
		return "", ""
	}
	switch p := rule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		return "GET", p.Get
	case *annotations.HttpRule_Put:
		return "PUT", p.Put
	case *annotations.HttpRule_Post:
		return "POST", p.Post
	case *annotations.HttpRule_Delete:
		return "DELETE", p.Delete
	case *annotations.HttpRule_Patch:
		return "PATCH", p.Patch
	case *annotations.HttpRule_Custom:
		if p.Custom == nil {
			return "", ""
		}
		return p.Custom.Kind, p.Custom.Path
	}
	return "", ""
}

func checkBamName(name string) error {
	if name == "" || !bamName.MatchString(name) || len(name) > 64 {
		return fmt.Errorf("bad module name")
	}
	return nil
}

func checkProtoDir(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return fmt.Errorf("proto 目录须是仓库内相对路径")
	}
	return nil
}

func protoModuleRoot(repo, protoDir string) string {
	d := filepath.Clean(protoDir)
	for {
		if repo != "" {
			if _, err := os.Stat(filepath.Join(repo, d, "buf.yaml")); err == nil {
				return filepath.ToSlash(d)
			}
		}
		parent := filepath.Dir(d)
		if parent == d || parent == "." {
			break
		}
		d = parent
	}
	slash := filepath.ToSlash(filepath.Clean(protoDir))
	if slash == "proto" || strings.HasPrefix(slash, "proto/") {
		return "proto"
	}
	return slash
}

func collectBamFiles(repo, protoDir string) ([]*v1.BamFile, error) {
	if err := checkProtoDir(protoDir); err != nil {
		return nil, err
	}
	absDir := filepath.Join(repo, filepath.FromSlash(protoDir))
	st, err := os.Stat(absDir)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("proto 目录不存在: %s", protoDir)
	}
	rootRel := protoModuleRoot(repo, protoDir)
	rootAbs := filepath.Join(repo, filepath.FromSlash(rootRel))
	var files []*v1.BamFile
	err = filepath.WalkDir(absDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "google/") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, &v1.BamFile{Path: rel, Content: string(b)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("目录 %s 下没有 .proto", protoDir)
	}
	return checkBamFiles(files)
}

func gitHead(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *server) syncBamRepo(scmName, branch string, w io.Writer) (string, error) {
	job, err := s.getJob(scmName)
	if err != nil {
		return "", err
	}
	if w == nil {
		w = io.Discard
	}
	return s.gitSync("bam-"+job.Name, job.GitURL, branch, w)
}

func checkBamFiles(files []*v1.BamFile) ([]*v1.BamFile, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("至少需要一个 .proto 文件")
	}
	if len(files) > 20 {
		return nil, fmt.Errorf("太多文件")
	}
	seen := map[string]bool{}
	var out []*v1.BamFile
	for _, f := range files {
		p := filepath.ToSlash(strings.TrimSpace(f.Path))
		p = strings.TrimPrefix(p, "/")
		if !bamPath.MatchString(p) || strings.Contains(p, "..") || strings.Contains(p, "//") {
			return nil, fmt.Errorf("bad proto path %s", f.Path)
		}
		if seen[p] {
			return nil, fmt.Errorf("duplicate %s", p)
		}
		seen[p] = true
		if len(f.Content) > 1<<20 {
			return nil, fmt.Errorf("%s too large", p)
		}
		out = append(out, &v1.BamFile{Path: p, Content: f.Content})
	}
	return out, nil
}

func scanBamModule(sc interface {
	Scan(dest ...any) error
}) (*v1.BamModule, error) {
	m := &v1.BamModule{}
	var t sql.NullTime
	if err := sc.Scan(&m.Id, &m.Name, &m.Version, &m.Rpcs, &m.HttpApis, &m.ScmName, &m.Branch, &m.ProtoDir, &m.GitCommit, &t); err != nil {
		return nil, err
	}
	if t.Valid {
		m.CreatedAt = fmtTime(t.Time)
	}
	return m, nil
}

func ensurePluginPath() {
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return
	}
	bin := filepath.Join(strings.TrimSpace(string(out)), "bin")
	os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func bamGenYAML(root string) (string, error) {
	var b strings.Builder
	b.WriteString("version: v2\nplugins:\n")
	b.WriteString("  - local: protoc-gen-go\n    out: gen\n    opt:\n      - paths=source_relative\n")
	b.WriteString("  - local: protoc-gen-connect-go\n    out: gen\n    opt:\n      - paths=source_relative\n")
	es := ""
	if _, err := exec.LookPath("protoc-gen-es"); err == nil {
		es = "protoc-gen-es"
	}
	web := filepath.Join(root, "platform", "web")
	if es == "" {
		if _, err := os.Stat(filepath.Join(web, "node_modules", "@bufbuild", "protoc-gen-es")); err == nil {
			b.WriteString("  - local: [\"npx\", \"--prefix\", \"" + filepath.ToSlash(web) + "\", \"protoc-gen-es\"]\n")
			b.WriteString("    out: gen/ts\n    opt:\n      - target=ts\n      - import_extension=none\n")
			return b.String(), nil
		}
	}
	if es == "" {
		if _, err := os.Stat("/opt/buf-es/node_modules/@bufbuild/protoc-gen-es"); err == nil {
			b.WriteString("  - local: [\"npx\", \"--prefix\", \"/opt/buf-es\", \"protoc-gen-es\"]\n")
			b.WriteString("    out: gen/ts\n    opt:\n      - target=ts\n      - import_extension=none\n")
			return b.String(), nil
		}
	}
	if es != "" {
		b.WriteString("  - local: protoc-gen-es\n    out: gen/ts\n    opt:\n      - target=ts\n      - import_extension=none\n")
	}
	return b.String(), nil
}

func copyGoogleAPI(root, protoDir string) error {
	src := filepath.Join(root, "proto", "google")
	if _, err := os.Stat(src); err != nil {
		return err
	}
	return copyTree(src, filepath.Join(protoDir, "google"))
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
}

func listRel(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

func zipDir(root string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fw, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = fw.Write(b)
		return err
	})
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
