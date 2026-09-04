package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "minikitex/gen/platform/v1"
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
		_, _ = db.Exec(
			`INSERT IGNORE INTO tlb_route (site_id, name, path_prefix, target) VALUES (?,?,?,?)`,
			consoleID, "platform-connect", "/platform.v1.PlatformService", "platform:8081",
		)
		return nil
	}
	for _, r := range []tlbRoute{
		{Name: "platform-api", Path: "/api", Target: "platform:8081"},
		{Name: "platform-connect", Path: "/platform.v1.PlatformService", Target: "platform:8081"},
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
	return connect.NewResponse(&v1.ListTlbSitesResponse{Sites: sites, Zone: tlbZone}), nil
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
		Id: st.ID, Name: st.Name, Host: st.Host, Routes: routes, Zone: tlbZone,
	}), nil
}

func (s *server) CreateTlbSite(_ context.Context, req *connect.Request[v1.CreateTlbSiteRequest]) (*connect.Response[v1.TlbSite], error) {
	name, err := checkTlbSub(req.Msg.Name)
	if err != nil {
		return nil, invalid(err)
	}
	host := tlbHost(name)
	res, err := s.db.Exec(`INSERT INTO tlb_site (name, host) VALUES (?,?)`, name, host)
	if err != nil {
		return nil, invalid(fmt.Errorf("create tlb site: %w", err))
	}
	id, _ := res.LastInsertId()
	return connect.NewResponse(&v1.TlbSite{Id: id, Name: name, Host: host}), nil
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
	up := make([]*v1.TlbUpstream, 0, len(list))
	for _, n := range list {
		up = append(up, &v1.TlbUpstream{Name: n, Target: n + ":" + tlbListenPort(n)})
	}
	return connect.NewResponse(&v1.ListTlbUpstreamsResponse{Upstreams: up}), nil
}
