package main

import (
	"errors"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// contentConfig 来自环境变量：
//
//	PAGES_HOST        页面专用域名（如 pages.lssl.work）。设置后 /pages/ 只在该域名下提供，主域名访问会被重定向过去。
//	PAGES_API_ORIGIN  页面 JS 允许 fetch 的读接口源，写进 CSP connect-src，默认 https://lssl.work。
//	PAGES_EXTRA_SRC   额外允许的脚本/样式/字体来源（空格分隔，例如 https://cdn.jsdelivr.net），默认无。
type contentConfig struct {
	PagesHost string
	APIOrigin string
	ExtraSrc  string
}

func contentConfigFromEnv() contentConfig {
	return contentConfig{
		PagesHost: strings.ToLower(strings.TrimSpace(os.Getenv("PAGES_HOST"))),
		APIOrigin: strings.TrimSpace(os.Getenv("PAGES_API_ORIGIN")),
		ExtraSrc:  strings.TrimSpace(os.Getenv("PAGES_EXTRA_SRC")),
	}
}

func (c contentConfig) apiOrigin() string {
	if c.APIOrigin == "" {
		return "https://lssl.work"
	}
	return c.APIOrigin
}

// pageCSP：页面只能从自身取资源、只能 fetch 读接口源，不能被嵌入、不能提交表单。
// 没有独立域名时（PAGES_HOST 为空）额外加 sandbox，让页面落在不透明源里，碰不到主站的 localStorage 等。
func (c contentConfig) pageCSP() string {
	extra := ""
	if c.ExtraSrc != "" {
		extra = " " + c.ExtraSrc
	}
	csp := "default-src 'none'" +
		"; script-src 'self' 'unsafe-inline'" + extra +
		"; style-src 'self' 'unsafe-inline'" + extra +
		"; font-src 'self' data:" + extra +
		"; img-src 'self' data: https:" +
		"; connect-src " + c.apiOrigin() +
		"; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'"
	if c.PagesHost == "" {
		csp += "; sandbox allow-scripts"
	}
	return csp
}

func requestHost(r *http.Request) string {
	h := r.Host
	if hh, _, err := net.SplitHostPort(h); err == nil {
		h = hh
	}
	return strings.ToLower(h)
}

// withPagesHost：请求打到页面专用域名时只交给页面处理器，绝不暴露 /mcp、/api/tokens 等。
func withPagesHost(cfg contentConfig, main, pages http.Handler) http.Handler {
	if cfg.PagesHost == "" {
		return main
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestHost(r) == cfg.PagesHost {
			pages.ServeHTTP(w, r)
			return
		}
		main.ServeHTTP(w, r)
	})
}

func readCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func writeStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, errNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func pathID(r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	return id, err == nil && id > 0
}

// registerContentRoutes 注册 v2 只读接口与页面托管。浏览器侧路径带 /board 前缀，由 TLB 剥掉。
func registerContentRoutes(mux *http.ServeMux, c ContentStore, cfg contentConfig) {
	mux.HandleFunc("OPTIONS /api/v2/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v2/modules", readCORS(func(w http.ResponseWriter, r *http.Request) {
		page, size := queryPage(r)
		p, err := c.ListChildren(0, NodeModule, page, size)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}))
	mux.HandleFunc("GET /api/v2/modules/{slug}/lists", readCORS(func(w http.ResponseWriter, r *http.Request) {
		m, err := c.FindChild(0, NodeModule, r.PathValue("slug"))
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		page, size := queryPage(r)
		p, err := c.ListChildren(m.ID, NodeList, page, size)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"module": m, "lists": p})
	}))
	mux.HandleFunc("GET /api/v2/lists/{id}/details", readCORS(func(w http.ResponseWriter, r *http.Request) {
		l, ok := typedNode(w, r, c, NodeList)
		if !ok {
			return
		}
		page, size := queryPage(r)
		p, err := c.ListChildren(l.ID, NodeDetail, page, size)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"list": l, "details": p})
	}))
	mux.HandleFunc("GET /api/v2/details/{id}", readCORS(func(w http.ResponseWriter, r *http.Request) {
		d, ok := typedNode(w, r, c, NodeDetail)
		if !ok {
			return
		}
		blocks := []Node{}
		for page := 1; page <= 10; page++ {
			p, err := c.ListChildren(d.ID, NodeBlock, page, 100)
			if err != nil {
				writeStoreErr(w, err)
				return
			}
			for _, b := range p.Items {
				blocks = append(blocks, b.brief())
			}
			if page*100 >= p.Total {
				break
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"detail": d, "blocks": blocks})
	}))
	mux.HandleFunc("GET /api/v2/details/{id}/blocks/{slug}", readCORS(func(w http.ResponseWriter, r *http.Request) {
		d, ok := typedNode(w, r, c, NodeDetail)
		if !ok {
			return
		}
		b, err := c.FindChild(d.ID, NodeBlock, r.PathValue("slug"))
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, b)
	}))

	mux.HandleFunc("GET /pages/{rest...}", func(w http.ResponseWriter, r *http.Request) {
		if cfg.PagesHost != "" {
			// 有独立域名时主域名不托管页面，避免与主站同源。
			http.Redirect(w, r, "https://"+cfg.PagesHost+"/"+r.PathValue("rest"), http.StatusFound)
			return
		}
		servePage(w, r, c, cfg, r.PathValue("rest"))
	})
}

func queryPage(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	return page, size
}

func typedNode(w http.ResponseWriter, r *http.Request, c ContentStore, typ string) (Node, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
		return Node{}, false
	}
	n, err := c.GetNode(id)
	if err == nil && n.Type != typ {
		err = errNotFound
	}
	if err != nil {
		writeStoreErr(w, err)
		return Node{}, false
	}
	return n, true
}

// pagesHandler 是页面专用域名下的处理器：只允许 GET/HEAD 读页面文件。
func pagesHandler(c ContentStore, cfg contentConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/" {
			// 根路径固定是导航页；页面路径至少 1 段，Agent 上传无法覆盖它。
			serveNav(w, r, c)
			return
		}
		servePage(w, r, c, cfg, strings.TrimPrefix(r.URL.Path, "/"))
	})
}

// servePage 把 rest（如 finance/drawdown/app.js）拆成「页面路径 + 文件名」，取 current 版本。
func servePage(w http.ResponseWriter, r *http.Request, c ContentStore, cfg contentConfig, rest string) {
	trailing := rest == "" || strings.HasSuffix(rest, "/")
	segs := strings.Split(strings.Trim(rest, "/"), "/")
	if len(segs) == 1 && segs[0] == "" {
		http.NotFound(w, r)
		return
	}
	for k := min(len(segs), maxPageDepth); k >= 1; k-- {
		p, err := cleanPagePath(strings.Join(segs[:k], "/"))
		if err != nil {
			continue
		}
		files, err := c.PageFiles(p)
		if errors.Is(err, errNotFound) {
			continue
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		name := strings.Join(segs[k:], "/")
		if name == "" {
			if !trailing {
				// 补上末尾斜杠，页面里的相对路径（./app.js）才能解析对。
				// 直接写相对 Location（http.Redirect 会转成绝对路径，丢掉 TLB 剥掉的 /board 前缀）。
				w.Header().Set("Location", segs[len(segs)-1]+"/")
				w.WriteHeader(http.StatusMovedPermanently)
				return
			}
			name = "index.html"
		}
		body, ok := files[name]
		if !ok {
			break
		}
		ct := mime.TypeByExtension(path.Ext(name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		h := w.Header()
		h.Set("Content-Type", ct)
		h.Set("Content-Security-Policy", cfg.pageCSP())
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-cache")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte(body))
		return
	}
	http.NotFound(w, r)
}
