package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "minikitex/gen/platform/v1"
)

const tlbZoneDefault = "ls-byte-basic.com"

var tlbPath = regexp.MustCompile(`^/[A-Za-z0-9._/-]*$`)
var tlbTarget = regexp.MustCompile(`^[A-Za-z0-9._-]+:[0-9]+$`)
var tlbSub = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
var tlbHostName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

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
		return "", fmt.Errorf("名称只允许小写字母数字和 -")
	}
	return name, nil
}

func tlbZone() string {
	if z := strings.TrimSpace(os.Getenv("TLB_ZONE")); z != "" {
		return strings.ToLower(z)
	}
	if z := strings.TrimSpace(os.Getenv("TLB_TLS_NAME")); z != "" {
		return strings.ToLower(z)
	}
	return tlbZoneDefault
}

func tlbHost(name string) string {
	return name + "." + tlbZone()
}

func checkTlbHost(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", fmt.Errorf("请填写域名")
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSuffix(p, ".")
		if strings.ContainsAny(p, "/:@") || !tlbHostName.MatchString(p) {
			return "", fmt.Errorf("域名写成 lssl.work 或 order.example.com")
		}
		out = append(out, p)
	}
	return strings.Join(out, " "), nil
}

func tlbNameFromHost(host string) string {
	host = strings.Fields(host)[0]
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}

func tlbSiteFromInput(name, host string) (string, string, error) {
	name = strings.TrimSpace(name)
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		n, err := checkTlbSub(name)
		if err != nil {
			return "", "", err
		}
		return n, tlbHost(n), nil
	}
	if !strings.Contains(host, ".") {
		sub, err := checkTlbSub(host)
		if err != nil {
			return "", "", err
		}
		if name == "" {
			name = sub
		}
		n, err := checkTlbSub(name)
		if err != nil {
			return "", "", err
		}
		return n, tlbHost(sub), nil
	}
	h, err := checkTlbHost(host)
	if err != nil {
		return "", "", err
	}
	if name == "" {
		name = tlbNameFromHost(h)
	}
	n, err := checkTlbSub(name)
	if err != nil {
		return "", "", err
	}
	return n, h, nil
}

func seedTlb(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tlb_site`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		host := tlbHost("console")
		if tls := strings.TrimSpace(os.Getenv("TLB_TLS_NAME")); tls != "" {
			host = strings.ToLower(tls)
		}
		if _, err := db.Exec(`INSERT INTO tlb_site (name, host) VALUES (?,?)`, "console", host); err != nil {
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
		_, _ = db.Exec(
			`INSERT IGNORE INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`,
			consoleID, "platform-connect", "/minikitex/platform.v1.PlatformService", "platform:8081",
		)
		migrateTlbMinikite(db)
		seedTlbHome(db, consoleID)
		seedTlbBoard(db, consoleID)
		return nil
	}
	for _, r := range []tlbRoute{
		{Name: "platform-api", Path: "/minikitex/api", Target: "platform:8081"},
		{Name: "platform-connect", Path: "/minikitex/platform.v1.PlatformService", Target: "platform:8081"},
		{Name: "order", Path: "/order", Target: "order:8080"},
		{Name: "platform-web", Path: "/minikitex", Target: "platform-web:80"},
		{Name: "home-api", Path: "/api", Target: "home:80"},
		{Name: "home-connect", Path: "/home.v1.HomeService", Target: "home:80"},
		{Name: "home-web", Path: "/", Target: "home-web:80"},
	} {
		if _, err := db.Exec(`INSERT INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`, consoleID, r.Name, r.Path, r.Target); err != nil {
			return err
		}
	}
	migrateTlbMinikite(db)
	seedTlbHome(db, consoleID)
	seedTlbBoard(db, consoleID)
	return nil
}

func migrateTlbMinikite(db *sql.DB) {
	_, _ = db.Exec(`UPDATE tlb_route SET path_prefix='/minikitex/api' WHERE name='platform-api' AND path_prefix IN ('/api','/minikite/api')`)
	_, _ = db.Exec(`UPDATE tlb_route SET path_prefix='/minikitex/platform.v1.PlatformService' WHERE name='platform-connect' AND path_prefix IN ('/platform.v1.PlatformService','/minikite/platform.v1.PlatformService')`)
	_, _ = db.Exec(`UPDATE tlb_route SET path_prefix='/minikitex' WHERE name='platform-web' AND path_prefix IN ('/','/minikite')`)
	_, _ = db.Exec(`UPDATE tlb_route SET target='platform-web:80' WHERE name='platform-web' AND target IN ('platform:8081','platform:80')`)
}

func seedTlbHome(db *sql.DB, consoleID int64) {
	_, _ = db.Exec(
		`INSERT IGNORE INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`,
		consoleID, "home-api", "/api", "home:80",
	)
	_, _ = db.Exec(
		`INSERT IGNORE INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`,
		consoleID, "home-connect", "/home.v1.HomeService", "home:80",
	)
	_, _ = db.Exec(
		`INSERT IGNORE INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`,
		consoleID, "home-web", "/", "home-web:80",
	)
	_, _ = db.Exec(`UPDATE tlb_route SET target='home:80' WHERE name IN ('home-api','home-connect') AND target='home:8082'`)
}

func seedTlbBoard(db *sql.DB, consoleID int64) {
	_, _ = db.Exec(
		`INSERT IGNORE INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`,
		consoleID, "board", "/board", "board:8090",
	)
}

func (s *server) getTlbSite(name string) (tlbSite, error) {
	var st tlbSite
	err := s.db.QueryRow(`SELECT id, name, host FROM tlb_site WHERE name=?`, name).Scan(&st.ID, &st.Name, &st.Host)
	if err != nil {
		return st, fmt.Errorf("no tlb site %s", name)
	}
	return st, nil
}

func (s *server) ListTlbSites(context.Context, *connect.Request[v1.ListTlbSitesRequest]) (*connect.Response[v1.ListTlbSitesResponse], error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.name, s.host, s.created_at, COUNT(r.id)
		FROM tlb_site s
		LEFT JOIN tlb_route r ON r.site_id = s.id
		GROUP BY s.id, s.name, s.host, s.created_at
		ORDER BY s.id`)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var sites []*v1.TlbSite
	for rows.Next() {
		st := &v1.TlbSite{}
		var t time.Time
		var n int32
		if err := rows.Scan(&st.Id, &st.Name, &st.Host, &t, &n); err != nil {
			return nil, internal(err)
		}
		st.CreatedAt = fmtTime(t)
		st.Routes = n
		sites = append(sites, st)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListTlbSitesResponse{Sites: sites, Zone: tlbZone()}), nil
}

func (s *server) ShowTlbSite(_ context.Context, req *connect.Request[v1.ShowTlbSiteRequest]) (*connect.Response[v1.TlbSiteDetail], error) {
	st, err := s.getTlbSite(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	rows, err := s.db.Query(`SELECT id, name, path_prefix, target, created_at FROM tlb_route WHERE site_id=? ORDER BY CHAR_LENGTH(path_prefix) DESC, id`, st.ID)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var routes []*v1.TlbRoute
	for rows.Next() {
		r := &v1.TlbRoute{}
		var t time.Time
		if err := rows.Scan(&r.Id, &r.Name, &r.Path, &r.Target, &t); err != nil {
			return nil, internal(err)
		}
		r.CreatedAt = fmtTime(t)
		routes = append(routes, r)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.TlbSiteDetail{
		Id: st.ID, Name: st.Name, Host: st.Host, Routes: routes, Zone: tlbZone(),
	}), nil
}

func (s *server) CreateTlbSite(_ context.Context, req *connect.Request[v1.CreateTlbSiteRequest]) (*connect.Response[v1.TlbSite], error) {
	name, host, err := tlbSiteFromInput(req.Msg.Name, req.Msg.Host)
	if err != nil {
		return nil, invalid(err)
	}
	res, err := s.db.Exec(`INSERT INTO tlb_site (name, host) VALUES (?,?)`, name, host)
	if err != nil {
		return nil, invalid(fmt.Errorf("create tlb site: %w", err))
	}
	id, _ := res.LastInsertId()
	return connect.NewResponse(&v1.TlbSite{Id: id, Name: name, Host: host}), nil
}

func (s *server) UpdateTlbSite(_ context.Context, req *connect.Request[v1.UpdateTlbSiteRequest]) (*connect.Response[v1.TlbSite], error) {
	st, err := s.getTlbSite(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	if strings.TrimSpace(req.Msg.Host) == "" {
		return nil, invalid(fmt.Errorf("请填写域名"))
	}
	_, host, err := tlbSiteFromInput(st.Name, req.Msg.Host)
	if err != nil {
		return nil, invalid(err)
	}
	if _, err := s.db.Exec(`UPDATE tlb_site SET host=? WHERE id=?`, host, st.ID); err != nil {
		return nil, invalid(fmt.Errorf("update tlb site: %w", err))
	}
	var n int32
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM tlb_route WHERE site_id=?`, st.ID).Scan(&n)
	return connect.NewResponse(&v1.TlbSite{Id: st.ID, Name: st.Name, Host: host, Routes: n}), nil
}

func (s *server) DeleteTlbSite(_ context.Context, req *connect.Request[v1.DeleteTlbSiteRequest]) (*connect.Response[v1.DeleteTlbSiteResponse], error) {
	st, err := s.getTlbSite(req.Msg.Name)
	if err != nil {
		return nil, notFound(err)
	}
	if _, err := s.db.Exec(`DELETE FROM tlb_route WHERE site_id=?`, st.ID); err != nil {
		return nil, internal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM tlb_site WHERE id=?`, st.ID); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.DeleteTlbSiteResponse{Name: st.Name}), nil
}

func tlbRouteName(name, path string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.Trim(path, "/")
		if name == "" {
			name = "root"
		}
	}
	if !jobName.MatchString(name) {
		return "", fmt.Errorf("bad route name")
	}
	return name, nil
}

func (s *server) CreateTlbRoute(_ context.Context, req *connect.Request[v1.CreateTlbRouteRequest]) (*connect.Response[v1.TlbRoute], error) {
	st, err := s.getTlbSite(req.Msg.Site)
	if err != nil {
		return nil, notFound(err)
	}
	path, err := checkTlbPath(req.Msg.Path)
	if err != nil {
		return nil, invalid(err)
	}
	target, err := checkTlbTarget(req.Msg.Target)
	if err != nil {
		return nil, invalid(err)
	}
	name, err := tlbRouteName(req.Msg.Name, path)
	if err != nil {
		return nil, invalid(err)
	}
	res, err := s.db.Exec(`INSERT INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`, st.ID, name, path, target)
	if err != nil {
		return nil, invalid(fmt.Errorf("create tlb route: %w", err))
	}
	id, _ := res.LastInsertId()
	return connect.NewResponse(&v1.TlbRoute{Id: id, Name: name, Path: path, Target: target}), nil
}

func (s *server) UpdateTlbRoute(_ context.Context, req *connect.Request[v1.UpdateTlbRouteRequest]) (*connect.Response[v1.TlbRoute], error) {
	st, err := s.getTlbSite(req.Msg.Site)
	if err != nil {
		return nil, notFound(err)
	}
	path, err := checkTlbPath(req.Msg.Path)
	if err != nil {
		return nil, invalid(err)
	}
	target, err := checkTlbTarget(req.Msg.Target)
	if err != nil {
		return nil, invalid(err)
	}
	name, err := tlbRouteName(req.Msg.Name, path)
	if err != nil {
		return nil, invalid(err)
	}
	res, err := s.db.Exec(`UPDATE tlb_route SET name=?, path_prefix=?, target=? WHERE id=? AND site_id=?`, name, path, target, req.Msg.Id, st.ID)
	if err != nil {
		return nil, invalid(fmt.Errorf("update tlb route: %w", err))
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, notFound(fmt.Errorf("no tlb route %d", req.Msg.Id))
	}
	return connect.NewResponse(&v1.TlbRoute{Id: req.Msg.Id, Name: name, Path: path, Target: target}), nil
}

func (s *server) DeleteTlbRoute(_ context.Context, req *connect.Request[v1.DeleteTlbRouteRequest]) (*connect.Response[v1.DeleteTlbRouteResponse], error) {
	st, err := s.getTlbSite(req.Msg.Site)
	if err != nil {
		return nil, notFound(err)
	}
	res, err := s.db.Exec(`DELETE FROM tlb_route WHERE id=? AND site_id=?`, req.Msg.Id, st.ID)
	if err != nil {
		return nil, internal(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, notFound(fmt.Errorf("no tlb route %d", req.Msg.Id))
	}
	return connect.NewResponse(&v1.DeleteTlbRouteResponse{Id: req.Msg.Id}), nil
}

func (s *server) PublishTlb(context.Context, *connect.Request[v1.PublishTlbRequest]) (*connect.Response[v1.PublishTlbResponse], error) {
	sites, err := s.loadTlb()
	if err != nil {
		return nil, internal(err)
	}
	if len(sites) == 0 {
		return nil, invalid(fmt.Errorf("没有配置"))
	}
	n := 0
	for _, st := range sites {
		n += len(st.routes)
	}
	if n == 0 {
		return nil, invalid(fmt.Errorf("没有路由"))
	}
	conf := renderTlbNginx(sites)
	cmd := s.composeCmd("exec", "-T", "tlb", "sh", "-c",
		"cat > /generated/nginx.conf && nginx -t -c /generated/nginx.conf && cp /generated/nginx.conf /etc/nginx/nginx.conf && nginx -s reload")
	cmd.Stdin = strings.NewReader(conf)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, internal(fmt.Errorf("nginx: %w\n%s", err, out))
	}
	return connect.NewResponse(&v1.PublishTlbResponse{Status: "ok", Sites: int32(len(sites)), Routes: int32(n)}), nil
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
		extra := ""
		if tlbTLSOn() {
			listen = "listen 443 ssl;"
			if i == 0 {
				listen = "listen 443 ssl default_server;"
			}
			extra = "\n        http2 on;"
		}
		fmt.Fprintf(&b, `
    server {
        %s%s
        server_name %s;
        client_max_body_size 32m;
`, listen, extra, st.Host)
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
            set $tlb_upstream %s;%s
            proxy_pass http://$tlb_upstream;
            proxy_http_version 1.1;
            proxy_set_header Host $http_host;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_buffering off;
            proxy_read_timeout 3600s;
        }
`, r.Path, r.Target, tlbStrip(r.Path))
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// tlbStrip 去掉这条路由配置的整段前缀。proxy_pass 用了变量，nginx 不会按 location 自动截。
// 配 /a/b/c，请求 /a/b/c/d，上游收到 /d。根路径 / 原样转发。
func tlbStrip(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" || path == "/" {
		return ""
	}
	return "\n            rewrite ^" + regexp.QuoteMeta(path) + "/?(.*)$ /$1 break;"
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

func composeServicePorts(raw string) map[string][]string {
	var cfg struct {
		Services map[string]json.RawMessage `json:"services"`
	}
	if json.Unmarshal([]byte(raw), &cfg) != nil || len(cfg.Services) == 0 {
		return nil
	}
	out := map[string][]string{}
	for name, svc := range cfg.Services {
		ps := serviceListenPorts(svc)
		if len(ps) == 0 {
			continue
		}
		out[name] = ps
	}
	return out
}

func serviceListenPorts(raw json.RawMessage) []string {
	var svc map[string]any
	if json.Unmarshal(raw, &svc) != nil {
		return nil
	}
	env := envMap(svc["environment"])
	skip := map[string]bool{}
	if p := hostPort(env["METRICS"]); p != "" {
		skip[p] = true
	}
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = hostPort(p)
		if p == "" || skip[p] || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	add(env["LISTEN"])
	if arr, ok := svc["expose"].([]any); ok {
		for _, x := range arr {
			add(fmt.Sprint(x))
		}
	}
	if arr, ok := svc["ports"].([]any); ok {
		for _, x := range arr {
			switch p := x.(type) {
			case map[string]any:
				add(jsonPort(p["target"]))
			default:
				add(fmt.Sprint(x))
			}
		}
	}
	return out
}

func envMap(v any) map[string]string {
	m := map[string]string{}
	switch e := v.(type) {
	case map[string]any:
		for k, val := range e {
			if val != nil {
				m[k] = fmt.Sprint(val)
			}
		}
	case []any:
		for _, x := range e {
			s, _ := x.(string)
			k, val, ok := strings.Cut(s, "=")
			if ok {
				m[k] = val
			}
		}
	}
	return m
}

func jsonPort(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.Itoa(int(x))
	case json.Number:
		return x.String()
	case string:
		return x
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func hostPort(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/tcp")
	s = strings.TrimSuffix(s, "/udp")
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return ""
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return ""
	}
	return strconv.Itoa(n)
}

func (s *server) ListTlbUpstreams(context.Context, *connect.Request[v1.ListTlbUpstreamsRequest]) (*connect.Response[v1.ListTlbUpstreamsResponse], error) {
	names := map[string]bool{}
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || tlbSkipSvc[n] || strings.HasPrefix(n, "user-") {
			return
		}
		names[n] = true
	}
	add("platform")
	add("platform-web")
	add("home")
	add("home-web")
	add("order")
	ports := map[string][]string{}
	if out, err := s.composeCmd("config", "--format", "json").Output(); err == nil {
		ports = composeServicePorts(string(out))
		for n := range ports {
			add(n)
		}
	} else if out, err := s.compose("config", "--services"); err == nil {
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
	up := make([]*v1.TlbUpstream, 0, len(list))
	for _, n := range list {
		ps := ports[n]
		if len(ps) == 0 {
			ps = []string{tlbListenPort(n)}
		}
		for _, p := range ps {
			up = append(up, &v1.TlbUpstream{Name: n, Target: n + ":" + p})
		}
	}
	return connect.NewResponse(&v1.ListTlbUpstreamsResponse{Upstreams: up}), nil
}
