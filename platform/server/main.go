package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	v1 "minikitex/gen/platform/v1"
	"minikitex/idl"

	"connectrpc.com/connect"
)

type Service struct {
	Name    string   `json:"name"`
	Bin     string   `json:"bin"`
	Pkg     string   `json:"pkg"`
	Compose []string `json:"compose"`
}

var catalog = []Service{
	{Name: "user", Bin: "user", Pkg: "./example/server", Compose: []string{"user"}},
	{Name: "order", Bin: "order", Pkg: "./example/order", Compose: []string{"order"}},
	{Name: "gateway", Bin: "gateway", Pkg: "./example/gateway", Compose: []string{"gateway"}},
	{Name: "etcdui", Bin: "etcdui", Pkg: "./example/etcdui", Compose: []string{"etcdui"}},
}

type server struct {
	root string
	db   *sql.DB
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dsn := getenv("MYSQL", "root:minikitex@tcp(127.0.0.1:3306)/minikitex?parseTime=true&charset=utf8mb4")
	db, err := waitDB(dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := migrate(db); err != nil {
		log.Fatal(err)
	}
	s := &server{root: root, db: db}
	addr := getenv("LISTEN", "127.0.0.1:8081")
	log.Fatal(s.serveHTTP(addr))
}

func waitDB(dsn string) (*sql.DB, error) {
	var last error
	for i := 0; i < 60; i++ {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		if err := db.Ping(); err == nil {
			return db, nil
		} else {
			last = err
			_ = db.Close()
			time.Sleep(time.Second)
		}
	}
	return nil, fmt.Errorf("mysql: %w", last)
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS scm_job (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(64) NOT NULL UNIQUE,
			repo_dir VARCHAR(512) NOT NULL,
			script_path VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS scm_build (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			service VARCHAR(64) NOT NULL,
			version VARCHAR(64) NOT NULL,
			bin_path VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL,
			log_text MEDIUMTEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agw_publish (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			idl_name VARCHAR(64) NOT NULL,
			content MEDIUMTEXT NOT NULL,
			routes_json MEDIUMTEXT NOT NULL,
			status VARCHAR(32) NOT NULL,
			log_text MEDIUMTEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS deploy_app (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(64) NOT NULL UNIQUE,
			scm_name VARCHAR(64) NOT NULL,
			compose VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS deploy_record (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			service VARCHAR(64) NOT NULL,
			version VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			log_text MEDIUMTEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tlb_site (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(64) NOT NULL UNIQUE,
			host VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tlb_route (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			site_id BIGINT NOT NULL DEFAULT 0,
			name VARCHAR(64) NOT NULL,
			path_prefix VARCHAR(255) NOT NULL,
			target VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`ALTER TABLE scm_job ADD COLUMN branch VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE scm_build ADD COLUMN branch VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE scm_build ADD COLUMN git_commit VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE scm_job ADD COLUMN label VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE tlb_route ADD COLUMN site_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE tlb_route DROP INDEX name`,
		`ALTER TABLE tlb_route DROP INDEX path_prefix`,
		`ALTER TABLE tlb_route ADD UNIQUE KEY tlb_route_site_path (site_id, path_prefix)`,
	} {
		if _, err := db.Exec(q); err != nil && !skipAlter(err) {
			return err
		}
	}
	if err := seedAdmin(db); err != nil {
		return err
	}
	return seedTlb(db)
}

func dupColumn(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}

func skipAlter(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate column") ||
		strings.Contains(s, "duplicate key name") ||
		strings.Contains(s, "check that column/key exists")
}

func (s *server) ListServices(context.Context, *connect.Request[v1.ListServicesRequest]) (*connect.Response[v1.ListServicesResponse], error) {
	out := make([]*v1.Service, 0, len(catalog))
	for _, c := range catalog {
		out = append(out, &v1.Service{Name: c.Name, Bin: c.Bin, Pkg: c.Pkg, Compose: c.Compose})
	}
	return connect.NewResponse(&v1.ListServicesResponse{Services: out}), nil
}

func (s *server) ListIdls(_ context.Context, _ *connect.Request[v1.ListIdlsRequest]) (*connect.Response[v1.ListIdlsResponse], error) {
	matches, err := filepath.Glob(filepath.Join(s.root, "idl", "*.thrift"))
	if err != nil {
		return nil, internal(err)
	}
	var out []*v1.Idl
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".thrift")
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, internal(err)
		}
		out = append(out, idlView(name, string(b)))
	}
	return connect.NewResponse(&v1.ListIdlsResponse{Idls: out}), nil
}

func (s *server) GetIdl(_ context.Context, req *connect.Request[v1.GetIdlRequest]) (*connect.Response[v1.Idl], error) {
	path, err := s.idlPath(req.Msg.Name)
	if err != nil {
		return nil, invalid(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, notFound(err)
	}
	return connect.NewResponse(idlView(req.Msg.Name, string(b))), nil
}

func (s *server) SaveIdl(_ context.Context, req *connect.Request[v1.SaveIdlRequest]) (*connect.Response[v1.Idl], error) {
	path, err := s.idlPath(req.Msg.Name)
	if err != nil {
		return nil, invalid(err)
	}
	if _, err := idl.ParseString(req.Msg.Content); err != nil {
		return nil, invalid(fmt.Errorf("idl parse: %w", err))
	}
	if err := os.WriteFile(path, []byte(req.Msg.Content), 0o644); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(idlView(req.Msg.Name, req.Msg.Content)), nil
}

func (s *server) idlPath(name string) (string, error) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("bad idl name")
	}
	return filepath.Join(s.root, "idl", name+".thrift"), nil
}

func idlView(name, content string) *v1.Idl {
	view := &v1.Idl{Name: name, Content: content}
	spec, err := idl.ParseString(content)
	if err != nil {
		view.ParseError = err.Error()
		return view
	}
	view.Service = spec.Service
	for _, m := range spec.Methods {
		req, _ := spec.Struct(m.Req)
		resp, _ := spec.Struct(m.Resp)
		item := &v1.IdlMethod{
			Name:       m.Name,
			Req:        m.Req,
			Resp:       m.Resp,
			HttpMethod: m.HTTPMethod,
			Uri:        m.URI,
			ReqFields:  fieldsOf(req),
			RespFields: fieldsOf(resp),
		}
		if m.URI != "" {
			view.HttpApis++
			if item.HttpMethod == "" {
				item.HttpMethod = "POST"
			}
		}
		view.Methods = append(view.Methods, item)
	}
	return view
}

func fieldsOf(st *idl.Struct) []*v1.Field {
	if st == nil {
		return nil
	}
	var out []*v1.Field
	for _, f := range st.Fields {
		out = append(out, &v1.Field{Id: int32(f.ID), Type: f.Type, Name: f.Name})
	}
	return out
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func lookup(name string) (Service, bool) {
	for _, s := range catalog {
		if s.Name == name {
			return s, true
		}
	}
	return Service{}, false
}
