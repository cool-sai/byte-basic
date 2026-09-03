package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const tlbZone = "ls-byte-basic.com"

var tlbPath = regexp.MustCompile(`^/[A-Za-z0-9._/-]*$`)
var tlbTarget = regexp.MustCompile(`^[A-Za-z0-9._-]+:[0-9]+$`)
var tlbSub = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

type tlbSite struct {
	ID   int64
	Name string
	Host string
}

type tlbRoute struct {
	ID     int64
	SiteID int64
	Name   string
	Path   string
	Target string
}

func checkTlbPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p != "/" {
		p = strings.TrimRight(p, "/")
	}
	if !tlbPath.MatchString(p) || strings.Contains(p, "//") {
		return "", fmt.Errorf("bad path")
	}
	return p, nil
}

func checkTlbTarget(t string) (string, error) {
	t = strings.TrimSpace(t)
	if !tlbTarget.MatchString(t) {
		return "", fmt.Errorf("target 写成 host:port，比如 platform:8081")
	}
	return t, nil
}

func checkTlbSub(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !tlbSub.MatchString(name) {
		return "", fmt.Errorf("三级域名只允许小写字母数字和 -")
	}
	return name, nil
}

func tlbHost(name string) string {
	return name + "." + tlbZone
}

func seedTlb(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tlb_site`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`INSERT INTO tlb_site (name, host) VALUES (?,?)`, "console", tlbHost("console")); err != nil {
			return err
		}
	}
	var consoleID int64
	if err := db.QueryRow(`SELECT id FROM tlb_site WHERE name=?`, "console").Scan(&consoleID); err != nil {
		if err := db.QueryRow(`SELECT id FROM tlb_site ORDER BY id LIMIT 1`).Scan(&consoleID); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`UPDATE tlb_route SET site_id=? WHERE site_id=0`, consoleID); err != nil {
		return err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tlb_route`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for _, r := range []tlbRoute{
		{Name: "platform-api", Path: "/api", Target: "platform:8081"},
		{Name: "order", Path: "/order", Target: "order:8080"},
		{Name: "platform-web", Path: "/", Target: "platform:8081"},
	} {
		if _, err := db.Exec(`INSERT INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`, consoleID, r.Name, r.Path, r.Target); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) getTlbSite(name string) (tlbSite, error) {
	var st tlbSite
	err := s.db.QueryRow(`SELECT id, name, host FROM tlb_site WHERE name=?`, name).Scan(&st.ID, &st.Name, &st.Host)
	if err != nil {
		return st, fmt.Errorf("no tlb site %s", name)
	}
	return st, nil
}

func (s *server) listTlbSites(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`
		SELECT s.id, s.name, s.host, s.created_at, COUNT(r.id)
		FROM tlb_site s
		LEFT JOIN tlb_route r ON r.site_id = s.id
		GROUP BY s.id, s.name, s.host, s.created_at
		ORDER BY s.id`)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	sites := scanRows(rows, "id", "name", "host", "createdAt", "routes")
	if sites == nil {
		sites = []map[string]any{}
	}
	writeJSON(w, map[string]any{"sites": sites, "zone": tlbZone})
}

func (s *server) showTlbSite(w http.ResponseWriter, r *http.Request) {
	st, err := s.getTlbSite(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	rows, err := s.db.Query(`SELECT id, name, path_prefix, target, created_at FROM tlb_route WHERE site_id=? ORDER BY CHAR_LENGTH(path_prefix) DESC, id`, st.ID)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	routes := scanRows(rows, "id", "name", "path", "target", "createdAt")
	if routes == nil {
		routes = []map[string]any{}
	}
	writeJSON(w, map[string]any{"id": st.ID, "name": st.Name, "host": st.Host, "routes": routes, "zone": tlbZone})
}

func (s *server) createTlbSite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	name, err := checkTlbSub(body.Name)
	if err != nil {
		fail(w, 400, err)
		return
	}
	host := tlbHost(name)
	_, err = s.db.Exec(`INSERT INTO tlb_site (name, host) VALUES (?,?)`, name, host)
	if err != nil {
		fail(w, 400, fmt.Errorf("create tlb site: %w", err))
		return
	}
	writeJSON(w, map[string]any{"name": name, "host": host})
}

func (s *server) deleteTlbSite(w http.ResponseWriter, r *http.Request) {
	st, err := s.getTlbSite(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM tlb_route WHERE site_id=?`, st.ID); err != nil {
		fail(w, 500, err)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM tlb_site WHERE id=?`, st.ID); err != nil {
		fail(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"name": st.Name})
}

func (s *server) createTlbRoute(w http.ResponseWriter, r *http.Request) {
	st, err := s.getTlbSite(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	var body struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Target string `json:"target"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	path, err := checkTlbPath(body.Path)
	if err != nil {
		fail(w, 400, err)
		return
	}
	target, err := checkTlbTarget(body.Target)
	if err != nil {
		fail(w, 400, err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = strings.Trim(path, "/")
		if name == "" {
			name = "root"
		}
	}
	if !jobName.MatchString(name) {
		fail(w, 400, fmt.Errorf("bad route name"))
		return
	}
	res, err := s.db.Exec(`INSERT INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`, st.ID, name, path, target)
	if err != nil {
		fail(w, 400, fmt.Errorf("create tlb route: %w", err))
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"id": id, "name": name, "path": path, "target": target})
}

func (s *server) updateTlbRoute(w http.ResponseWriter, r *http.Request) {
	st, err := s.getTlbSite(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, 400, fmt.Errorf("bad id"))
		return
	}
	var body struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Target string `json:"target"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	path, err := checkTlbPath(body.Path)
	if err != nil {
		fail(w, 400, err)
		return
	}
	target, err := checkTlbTarget(body.Target)
	if err != nil {
		fail(w, 400, err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = strings.Trim(path, "/")
		if name == "" {
			name = "root"
		}
	}
	if !jobName.MatchString(name) {
		fail(w, 400, fmt.Errorf("bad route name"))
		return
	}
	res, err := s.db.Exec(`UPDATE tlb_route SET name=?, path_prefix=?, target=? WHERE id=? AND site_id=?`, name, path, target, id, st.ID)
	if err != nil {
		fail(w, 400, fmt.Errorf("update tlb route: %w", err))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, fmt.Errorf("no tlb route %d", id))
		return
	}
	writeJSON(w, map[string]any{"id": id, "name": name, "path": path, "target": target})
}

func (s *server) deleteTlbRoute(w http.ResponseWriter, r *http.Request) {
	st, err := s.getTlbSite(r.PathValue("name"))
	if err != nil {
		fail(w, 404, err)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, 400, fmt.Errorf("bad id"))
		return
	}
	res, err := s.db.Exec(`DELETE FROM tlb_route WHERE id=? AND site_id=?`, id, st.ID)
	if err != nil {
		fail(w, 500, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, fmt.Errorf("no tlb route %d", id))
		return
	}
	writeJSON(w, map[string]any{"id": id})
}

func (s *server) publishTlb(w http.ResponseWriter, _ *http.Request) {
	sites, err := s.loadTlb()
	if err != nil {
		fail(w, 500, err)
		return
	}
	if len(sites) == 0 {
		fail(w, 400, fmt.Errorf("没有配置"))
		return
	}
	n := 0
	for _, st := range sites {
		n += len(st.routes)
	}
	if n == 0 {
		fail(w, 400, fmt.Errorf("没有路由"))
		return
	}
	conf := renderTlbNginx(sites)
	cmd := s.composeCmd("exec", "-T", "tlb", "sh", "-c",
		"cat > /generated/nginx.conf && nginx -t -c /generated/nginx.conf && cp /generated/nginx.conf /etc/nginx/nginx.conf && nginx -s reload")
	cmd.Stdin = strings.NewReader(conf)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fail(w, 500, fmt.Errorf("nginx: %w\n%s", err, out))
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "sites": len(sites), "routes": n})
}

type tlbSiteFull struct {
	tlbSite
	routes []tlbRoute
}

func (s *server) loadTlb() ([]tlbSiteFull, error) {
	rows, err := s.db.Query(`SELECT id, name, host FROM tlb_site ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []tlbSiteFull
	idx := map[int64]int{}
	for rows.Next() {
		var st tlbSiteFull
		if err := rows.Scan(&st.ID, &st.Name, &st.Host); err != nil {
			return nil, err
		}
		idx[st.ID] = len(sites)
		sites = append(sites, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rrows, err := s.db.Query(`SELECT id, site_id, name, path_prefix, target FROM tlb_route`)
	if err != nil {
		return nil, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var r tlbRoute
		if err := rrows.Scan(&r.ID, &r.SiteID, &r.Name, &r.Path, &r.Target); err != nil {
			return nil, err
		}
		i, ok := idx[r.SiteID]
		if !ok {
			continue
		}
		sites[i].routes = append(sites[i].routes, r)
	}
	return sites, rrows.Err()
}

func renderTlbNginx(sites []tlbSiteFull) string {
	var b strings.Builder
	b.WriteString(`worker_processes auto;
error_log /dev/stderr warn;
pid /run/nginx/nginx.pid;
events {
    worker_connections 1024;
}
http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    sendfile on;
    resolver 127.0.0.11 valid=10s ipv6=off;
`)
	if tlbTLSOn() {
		b.WriteString(`
    server {
        listen 80 default_server;
        server_name _;
        location /.well-known/acme-challenge/ {
            root /acme;
        }
        location / {
            return 301 https://$host$request_uri;
        }
    }
`)
	}
	for i, st := range sites {
		if len(st.routes) == 0 {
			continue
		}
		listen := "listen 80;"
		if i == 0 {
			listen = "listen 80 default_server;"
		}
		if tlbTLSOn() {
			listen = "listen 443 ssl;"
			if i == 0 {
				listen = "listen 443 ssl default_server;"
			}
		}
		fmt.Fprintf(&b, `
    server {
        %s
        server_name %s;
        client_max_body_size 32m;
`, listen, st.Host)
		if tlbTLSOn() {
			dir := tlbTLSDir()
			fmt.Fprintf(&b, `        ssl_certificate %s/fullchain.pem;
        ssl_certificate_key %s/privkey.pem;
        ssl_protocols TLSv1.2 TLSv1.3;
`, dir, dir)
		} else {
			b.WriteString(`        location /.well-known/acme-challenge/ {
            root /acme;
        }
`)
		}
		routes := append([]tlbRoute(nil), st.routes...)
		sort.Slice(routes, func(a, b int) bool {
			return len(routes[a].Path) > len(routes[b].Path)
		})
		for _, r := range routes {
			fmt.Fprintf(&b, `
        location %s {
            set $tlb_upstream %s;
            proxy_pass http://$tlb_upstream;
            proxy_http_version 1.1;
            proxy_set_header Host $http_host;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_buffering off;
            proxy_read_timeout 3600s;
        }
`, r.Path, r.Target)
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func tlbTLSDir() string {
	name := strings.TrimSpace(os.Getenv("TLB_TLS_NAME"))
	if name == "" {
		return ""
	}
	return "/etc/letsencrypt/live/" + name
}

func tlbTLSOn() bool {
	dir := tlbTLSDir()
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "fullchain.pem"))
	return err == nil
}

var tlbSkipSvc = map[string]bool{
	"tlb": true, "mysql": true, "consul": true, "etcd": true,
	"jaeger": true, "loki": true, "promtail": true, "prometheus": true,
	"grafana": true, "adminer": true, "gateway": true,
}

func tlbListenPort(name string) string {
	switch name {
	case "platform":
		return "8081"
	case "order":
		return "8080"
	case "etcdui":
		return "2381"
	default:
		return "80"
	}
}

func (s *server) listTlbUpstreams(w http.ResponseWriter, _ *http.Request) {
	names := map[string]bool{}
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || tlbSkipSvc[n] || strings.HasPrefix(n, "user-") {
			return
		}
		names[n] = true
	}
	add("platform")
	add("order")
	if out, err := s.compose("config", "--services"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			add(line)
		}
	}
	if rows, err := s.db.Query(`SELECT compose FROM deploy_app`); err == nil {
		for rows.Next() {
			var compose string
			if err := rows.Scan(&compose); err != nil {
				continue
			}
			for _, c := range parseCompose(compose) {
				add(c)
			}
		}
		rows.Close()
	}
	list := make([]string, 0, len(names))
	for n := range names {
		list = append(list, n)
	}
	sort.Strings(list)
	up := make([]map[string]any, 0, len(list))
	for _, n := range list {
		port := tlbListenPort(n)
		up = append(up, map[string]any{"name": n, "target": n + ":" + port})
	}
	writeJSON(w, map[string]any{"upstreams": up})
}
