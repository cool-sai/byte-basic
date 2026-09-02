package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"minikitex/idl"
)

type Service struct {
	Name    string   `json:"name"`
	Bin     string   `json:"bin"`
	Pkg     string   `json:"pkg"`
	Compose []string `json:"compose"`
}

var catalog = []Service{
	{Name: "user", Bin: "user", Pkg: "./example/server", Compose: []string{"user-1", "user-2"}},
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/services", s.getServices)
	mux.HandleFunc("GET /api/scm/jobs", s.listJobs)
	mux.HandleFunc("POST /api/scm/jobs", s.createJob)
	mux.HandleFunc("GET /api/scm/builds", s.listBuilds)
	mux.HandleFunc("GET /api/scm/builds/{id}", s.getBuild)
	mux.HandleFunc("POST /api/scm/builds", s.createBuild)
	mux.HandleFunc("GET /api/bam/idls", s.listIDLs)
	mux.HandleFunc("GET /api/bam/idls/{name}", s.getIDL)
	mux.HandleFunc("PUT /api/bam/idls/{name}", s.saveIDL)
	mux.HandleFunc("GET /api/agw/publishes", s.listPublishes)
	mux.HandleFunc("POST /api/agw/publish", s.publishAGW)
	mux.HandleFunc("GET /api/deploys", s.listDeploys)
	mux.HandleFunc("POST /api/deploys", s.createDeploy)
	mux.HandleFunc("GET /api/runtime", s.runtime)
	mux.HandleFunc("GET /api/db/tables", s.listTables)
	mux.HandleFunc("GET /api/db/tables/{name}", s.getTable)

	if web := getenv("WEB_DIR", ""); web != "" {
		mux.Handle("/", spa(web))
	}

	addr := getenv("LISTEN", "127.0.0.1:8081")
	log.Println("platform", addr, "root", root, "web", getenv("WEB_DIR", ""))
	log.Fatal(http.ListenAndServe(addr, cors(mux)))
}

func spa(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		p := filepath.Join(dir, rel)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
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
		`CREATE TABLE IF NOT EXISTS deploy_record (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			service VARCHAR(64) NOT NULL,
			version VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			log_text MEDIUMTEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) getServices(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, catalog)
}

func (s *server) listIDLs(w http.ResponseWriter, _ *http.Request) {
	matches, err := filepath.Glob(filepath.Join(s.root, "idl", "*.thrift"))
	if err != nil {
		fail(w, 500, err)
		return
	}
	var out []map[string]any
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".thrift")
		b, err := os.ReadFile(path)
		if err != nil {
			fail(w, 500, err)
			return
		}
		view, err := idlView(name, string(b))
		if err != nil {
			out = append(out, map[string]any{"name": name, "parseError": err.Error(), "content": string(b)})
			continue
		}
		out = append(out, view)
	}
	writeJSON(w, out)
}

func (s *server) getIDL(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	path, err := s.idlPath(name)
	if err != nil {
		fail(w, 400, err)
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fail(w, 404, err)
		return
	}
	view, err := idlView(name, string(b))
	if err != nil {
		writeJSON(w, map[string]any{"name": name, "parseError": err.Error(), "content": string(b)})
		return
	}
	writeJSON(w, view)
}

func (s *server) saveIDL(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	path, err := s.idlPath(name)
	if err != nil {
		fail(w, 400, err)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, err)
		return
	}
	if _, err := idl.ParseString(body.Content); err != nil {
		fail(w, 400, fmt.Errorf("idl parse: %w", err))
		return
	}
	if err := os.WriteFile(path, []byte(body.Content), 0o644); err != nil {
		fail(w, 500, err)
		return
	}
	view, _ := idlView(name, body.Content)
	writeJSON(w, view)
}

func (s *server) idlPath(name string) (string, error) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("bad idl name")
	}
	return filepath.Join(s.root, "idl", name+".thrift"), nil
}

func idlView(name, content string) (map[string]any, error) {
	spec, err := idl.ParseString(content)
	if err != nil {
		return nil, err
	}
	var methods []map[string]any
	httpN := 0
	for _, m := range spec.Methods {
		req, _ := spec.Struct(m.Req)
		resp, _ := spec.Struct(m.Resp)
		item := map[string]any{
			"name":       m.Name,
			"req":        m.Req,
			"resp":       m.Resp,
			"httpMethod": m.HTTPMethod,
			"uri":        m.URI,
			"reqFields":  fieldsOf(req),
			"respFields": fieldsOf(resp),
		}
		if m.URI != "" {
			httpN++
			if item["httpMethod"] == "" {
				item["httpMethod"] = "POST"
			}
		}
		methods = append(methods, item)
	}
	return map[string]any{
		"name":     name,
		"service":  spec.Service,
		"content":  content,
		"methods":  methods,
		"httpApis": httpN,
	}, nil
}

func fieldsOf(st *idl.Struct) []map[string]any {
	if st == nil {
		return nil
	}
	var out []map[string]any
	for _, f := range st.Fields {
		out = append(out, map[string]any{"id": f.ID, "type": f.Type, "name": f.Name})
	}
	return out
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
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
