package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//go:embed web/index.html
var indexHTML []byte

//go:embed web/tokens.html
var tokensHTML []byte

//go:embed web/tools.html
var toolsHTML []byte

//go:embed web/kinds.html
var kindsHTML []byte

func main() {
	token := strings.TrimSpace(os.Getenv("BOARD_TOKEN"))
	if token == "" {
		log.Fatal("BOARD_TOKEN is required")
	}
	listen := strings.TrimSpace(os.Getenv("LISTEN"))
	if listen == "" {
		listen = "0.0.0.0:8090"
	}
	tokensFile := strings.TrimSpace(os.Getenv("TOKENS_FILE"))
	if tokensFile == "" {
		tokensFile = "/data/tokens.json"
	}
	dsn := strings.TrimSpace(os.Getenv("MYSQL"))
	if dsn == "" {
		log.Fatal("MYSQL is required")
	}

	board, err := openMySQL(dsn)
	if err != nil {
		log.Fatal(err)
	}
	tokens, err := NewTokenStore(tokensFile)
	if err != nil {
		log.Fatal(err)
	}
	content, err := newMySQLContent(board.db)
	if err != nil {
		log.Fatal(err)
	}
	cfg := contentConfigFromEnv()
	log.Printf("board %s token=%s tokens=%s pages_host=%q", listen, redact(token), tokensFile, cfg.PagesHost)
	log.Fatal(http.ListenAndServe(listen, newContentMux(token, board, tokens, content, cfg)))
}

// newMux 只给旧测试用：内容树用内存实现，页面不走独立域名。
func newMux(admin string, board Board, tokens *TokenStore) http.Handler {
	return newContentMux(admin, board, tokens, newMemContent(), contentConfig{})
}

func newContentMux(admin string, board Board, tokens *TokenStore, content ContentStore, cfg contentConfig) http.Handler {
	mux := http.NewServeMux()
	registerContentRoutes(mux, content, cfg)
	// 浏览器地址仍是 /board/...。TLB 剥掉这段前缀后，进程只看到后面的路径。
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /api/kv/stats", func(w http.ResponseWriter, _ *http.Request) {
		sum, err := board.Summary()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, sum)
	})
	mux.HandleFunc("GET /{$}", serveIndex)
	mux.HandleFunc("GET /tokens", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/board/tokens/", http.StatusFound)
	})
	mux.HandleFunc("GET /tokens/{$}", serveTokens)
	mux.HandleFunc("GET /tools", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/board/tools/", http.StatusFound)
	})
	mux.HandleFunc("GET /tools/{$}", serveTools)
	mux.HandleFunc("GET /kinds", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/board/kinds/", http.StatusFound)
	})
	mux.HandleFunc("GET /kinds/{$}", serveKinds)
	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, r *http.Request) {
		kind := strings.TrimSpace(r.URL.Query().Get("kind"))
		if kind == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind required"})
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		p, err := board.ListItems(kind, page, size)
		if err != nil {
			if strings.Contains(err.Error(), "没有种类") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, p)
	})
	server := newMCPServer(board)
	addContentTools(server, content)
	mux.HandleFunc("GET /api/tools", func(w http.ResponseWriter, _ *http.Request) {
		docs, err := catalogFromMCP(server)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"tools": docs})
	})
	adminOK := func(got string) bool { return matchAdmin(admin, got) }
	mux.Handle("GET /api/tokens", requireBearer(adminOK, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens.List()})
	})))
	mux.Handle("POST /api/tokens", requireBearer(adminOK, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		raw, view, err := tokens.Create(in.Name)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": view.ID, "name": view.Name, "prefix": view.Prefix,
			"created_at": view.CreatedAt, "token": raw,
		})
	})))
	mux.Handle("DELETE /api/tokens/{id}", requireBearer(adminOK, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := tokens.Delete(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})))
	mcpOK := func(got string) bool {
		if adminOK(got) {
			return true
		}
		return tokens.Match(got)
	}
	mcpH := requireBearer(mcpOK, newMCPHandler(server))
	mux.Handle("/mcp", mcpH)
	mux.Handle("/mcp/", mcpH)
	return withPagesHost(cfg, mux, pagesHandler(content, cfg))
}

func serveIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(indexHTML)
}

func serveTokens(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(tokensHTML)
}

func serveTools(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(toolsHTML)
}

func serveKinds(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(kindsHTML)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func redact(token string) string {
	if len(token) <= 6 {
		return "***"
	}
	return token[:4] + "…"
}
