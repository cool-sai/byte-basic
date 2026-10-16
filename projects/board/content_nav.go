package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

type navPage struct {
	Path  string
	Title string
}

type navModule struct {
	Slug        string
	Title       string
	Description string
	Pages       []navPage
}

type navView struct {
	Modules []navModule
	Orphans []navPage // 不属于任何已登记模块的页面
}

var navTmpl = template.Must(template.New("nav").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>页面导航</title>
<style>
*{box-sizing:border-box}body{margin:0;font:15px/1.6 -apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif;background:#f5f6f8;color:#1f2329}
main{max-width:760px;margin:0 auto;padding:28px 16px 48px}h1{font-size:22px;margin:0 0 4px}.sub{color:#8a9099;margin:0 0 20px;font-size:13px}
.card{background:#fff;border-radius:10px;padding:16px 18px;margin-bottom:12px;box-shadow:0 1px 2px rgba(0,0,0,.06)}
.card h2{font-size:17px;margin:0}.slug{color:#8a9099;font-size:12px;font-weight:normal;margin-left:6px}
.desc{color:#5c6370;margin:4px 0 0;font-size:14px}ul{list-style:none;margin:10px 0 0;padding:0}
li a{display:block;padding:9px 12px;margin-top:6px;border-radius:8px;background:#f0f4ff;color:#2b5fd9;text-decoration:none;word-break:break-all}
li a:hover{background:#e2eaff}.empty{color:#a0a6ae;font-size:13px;margin:10px 0 0}.none{text-align:center;color:#8a9099;padding:48px 0}
</style></head><body><main>
<h1>页面导航</h1><p class="sub">各 Agent 发布的展示页面</p>
{{if and (not .Modules) (not .Orphans)}}<div class="none">还没有任何模块或页面</div>{{end}}
{{range .Modules}}<section class="card"><h2>{{.Title}}<span class="slug">{{.Slug}}</span></h2>
{{if .Description}}<p class="desc">{{.Description}}</p>{{end}}
{{if .Pages}}<ul>{{range .Pages}}<li><a href="/{{.Path}}/">{{.Title}}</a></li>{{end}}</ul>{{else}}<p class="empty">暂无页面</p>{{end}}
</section>{{end}}
{{if .Orphans}}<section class="card"><h2>其他页面</h2><ul>{{range .Orphans}}<li><a href="/{{.Path}}/">{{.Title}}</a></li>{{end}}</ul></section>{{end}}
</main></body></html>`))

// navCSP：导航页是服务端渲染的纯 HTML，不需要任何脚本。
const navCSP = "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

func moduleDescription(data json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	for _, k := range []string{"description", "desc"} {
		if s, ok := m[k].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func buildNav(c ContentStore) (navView, error) {
	v := navView{Modules: []navModule{}}
	paths, err := c.ListPagePaths()
	if err != nil {
		return v, err
	}
	byMod := map[string][]navPage{}
	for _, p := range paths {
		mod, _, _ := strings.Cut(p, "/")
		byMod[mod] = append(byMod[mod], navPage{Path: p, Title: p})
	}
	for page := 1; page <= 20; page++ {
		res, err := c.ListChildren(0, NodeModule, page, 100)
		if err != nil {
			return v, err
		}
		for _, n := range res.Items {
			full, err := c.GetNode(n.ID)
			if err == nil {
				n = full
			}
			title := n.Title
			if title == "" {
				title = n.Slug
			}
			v.Modules = append(v.Modules, navModule{Slug: n.Slug, Title: title, Description: moduleDescription(n.Data), Pages: byMod[n.Slug]})
			delete(byMod, n.Slug)
		}
		if len(res.Items) == 0 || page*res.PageSize >= res.Total {
			break
		}
	}
	for _, p := range paths {
		mod, _, _ := strings.Cut(p, "/")
		if _, ok := byMod[mod]; ok {
			v.Orphans = append(v.Orphans, navPage{Path: p, Title: p})
		}
	}
	return v, nil
}

func serveNav(w http.ResponseWriter, r *http.Request, c ContentStore) {
	v, err := buildNav(c)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", navCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-cache")
	if r.Method == http.MethodHead {
		return
	}
	_ = navTmpl.Execute(w, v)
}
