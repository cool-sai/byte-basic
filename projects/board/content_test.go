package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mustPut(t *testing.T, c ContentStore, in NodeInput) Node {
	t.Helper()
	n, err := c.PutNode(in)
	if err != nil {
		t.Fatalf("put %+v: %v", in, err)
	}
	return n
}

// seedTree 建一棵 finance/runs/r1/{triggers,funds}。
func seedTree(t *testing.T, c ContentStore) (mod, list, det Node) {
	mod = mustPut(t, c, NodeInput{Type: NodeModule, Slug: "finance", Title: "理财"})
	list = mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeList, Slug: "runs", Title: "每日运行"})
	det = mustPut(t, c, NodeInput{ParentID: list.ID, Type: NodeDetail, Slug: "2026-10-16-r19", Data: json.RawMessage(`{"run_id":19}`)})
	mustPut(t, c, NodeInput{ParentID: det.ID, Type: NodeBlock, Slug: "triggers", Data: json.RawMessage(`{"hs300":17.2}`), SchemaVersion: 2})
	mustPut(t, c, NodeInput{ParentID: det.ID, Type: NodeBlock, Slug: "funds", Data: json.RawMessage(`[1,2]`)})
	return
}

func TestNodeHierarchyRules(t *testing.T) {
	c := newMemContent()
	mod, list, det := seedTree(t, c)
	bad := []NodeInput{
		{Type: NodeList, Slug: "x"},                         // list 不能挂根
		{ParentID: mod.ID, Type: NodeModule, Slug: "x"},     // module 不能有父
		{ParentID: mod.ID, Type: NodeDetail, Slug: "x"},     // detail 父必须是 list
		{ParentID: list.ID, Type: NodeBlock, Slug: "x"},     // block 父必须是 detail
		{ParentID: det.ID, Type: "page", Slug: "x"},         // 未知类型
		{Type: NodeModule, Slug: "Bad Slug"},                // slug 非法
		{Type: NodeModule, Slug: "m", Data: []byte(`{bad`)}, // 非法 JSON
		{ParentID: 9999, Type: NodeList, Slug: "x"},         // 父不存在
		{Type: NodeModule, Slug: "m2", SchemaVersion: -1},
	}
	for _, in := range bad {
		if _, err := c.PutNode(in); err == nil {
			t.Fatalf("expected error for %+v", in)
		}
	}
}

func TestPutNodeUpsertAndSoftDeleteSubtree(t *testing.T) {
	c := newMemContent()
	mod, list, det := seedTree(t, c)
	again := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeList, Slug: "runs", Title: "运行记录"})
	if again.ID != list.ID || again.Title != "运行记录" {
		t.Fatalf("upsert should keep id: %+v", again)
	}
	n, err := c.DeleteNode(list.ID)
	if err != nil || n != 4 { // list + detail + 2 blocks
		t.Fatalf("delete count %d err %v", n, err)
	}
	if _, err := c.GetNode(det.ID); err != errNotFound {
		t.Fatalf("detail should be gone: %v", err)
	}
	if p, _ := c.ListChildren(mod.ID, NodeList, 1, 20); p.Total != 0 {
		t.Fatalf("lists left: %+v", p)
	}
	if _, err := c.DeleteNode(list.ID); err != errNotFound {
		t.Fatalf("double delete: %v", err)
	}
	// 复活：同 parent+slug 再 put
	back := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeList, Slug: "runs"})
	if back.ID != list.ID {
		t.Fatalf("revive should reuse id")
	}
	title := "新"
	sv := 3
	u, err := c.UpdateNode(back.ID, &title, nil, &sv)
	if err != nil || u.Title != "新" || u.SchemaVersion != 3 {
		t.Fatalf("update %+v %v", u, err)
	}
}

func TestListChildrenPaging(t *testing.T) {
	c := newMemContent()
	_, list, _ := seedTree(t, c)
	for i := 0; i < 25; i++ {
		mustPut(t, c, NodeInput{ParentID: list.ID, Type: NodeDetail, Slug: "d" + string(rune('a'+i))})
	}
	p, _ := c.ListChildren(list.ID, NodeDetail, 2, 10)
	if p.Total != 26 || len(p.Items) != 10 || p.Page != 2 {
		t.Fatalf("page %+v", p)
	}
	p, _ = c.ListChildren(list.ID, NodeDetail, 1, 500)
	if p.PageSize != 100 {
		t.Fatalf("size cap %d", p.PageSize)
	}
	if first, _ := c.ListChildren(list.ID, NodeDetail, 1, 1); first.Items[0].Slug != "dy" {
		t.Fatalf("newest first, got %s", first.Items[0].Slug)
	}
}

func TestPageValidation(t *testing.T) {
	for _, p := range []string{"", "/", "A/b", "a/../b", "a/b/c/d/e", "a b"} {
		if _, err := cleanPagePath(p); err == nil {
			t.Fatalf("path %q should fail", p)
		}
	}
	for _, f := range []string{"../x", "/etc/passwd", "a//b", "a/./b", ".env", "a\\b", "a/b/c/d"} {
		if _, err := cleanPageFileName(f); err == nil {
			t.Fatalf("file %q should fail", f)
		}
	}
	if _, err := cleanPageFiles(map[string]string{"app.js": "x"}); err == nil {
		t.Fatal("index.html required")
	}
	if _, err := cleanPageFiles(map[string]string{"index.html": strings.Repeat("x", maxPageBytes+1)}); err == nil {
		t.Fatal("size limit")
	}
}

func TestPageVersionsAndRollback(t *testing.T) {
	c := newMemContent()
	if _, err := c.PutPage("finance/drawdown", map[string]string{"index.html": "v1"}); err != nil {
		t.Fatal(err)
	}
	v2, _ := c.PutPage("finance/drawdown", map[string]string{"index.html": "v2", "app.js": "x"})
	if v2.Version != 2 {
		t.Fatalf("v2 %+v", v2)
	}
	files, _ := c.PageFiles("finance/drawdown")
	if files["index.html"] != "v2" {
		t.Fatal("current should be v2")
	}
	if _, err := c.RollbackPage("finance/drawdown", 1); err != nil {
		t.Fatal(err)
	}
	files, _ = c.PageFiles("finance/drawdown")
	if files["index.html"] != "v1" {
		t.Fatal("rollback failed")
	}
	vs, _ := c.ListPageVersions("finance/drawdown")
	if len(vs) != 2 || vs[0].Version != 2 || vs[0].Current || !vs[1].Current {
		t.Fatalf("versions %+v", vs)
	}
	if _, err := c.RollbackPage("finance/drawdown", 9); err != errNotFound {
		t.Fatalf("bad version: %v", err)
	}
}

func contentServer(t *testing.T, cfg contentConfig) (*httptest.Server, *memContent) {
	t.Helper()
	ts, err := NewTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := newMemContent()
	srv := httptest.NewServer(newContentMux("secret", newMemBoard(), ts, c, cfg))
	t.Cleanup(srv.Close)
	return srv, c
}

func getJSON(t *testing.T, url string, out any) *http.Response {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil && res.StatusCode == 200 {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

func TestReadAPI(t *testing.T) {
	srv, c := contentServer(t, contentConfig{})
	_, list, det := seedTree(t, c)
	var mods NodePage
	res := getJSON(t, srv.URL+"/api/v2/modules", &mods)
	if res.Header.Get("Access-Control-Allow-Origin") != "*" || mods.Total != 1 || mods.Items[0].Slug != "finance" {
		t.Fatalf("modules %+v %v", mods, res.Header)
	}
	var lists struct{ Lists NodePage }
	getJSON(t, srv.URL+"/api/v2/modules/finance/lists", &lists)
	if lists.Lists.Total != 1 || lists.Lists.Items[0].ID != list.ID {
		t.Fatalf("lists %+v", lists)
	}
	var dets struct{ Details NodePage }
	getJSON(t, srv.URL+"/api/v2/lists/"+itoa(list.ID)+"/details?page=1&size=5", &dets)
	if dets.Details.Total != 1 || dets.Details.PageSize != 5 {
		t.Fatalf("details %+v", dets)
	}
	var detail struct {
		Detail Node
		Blocks []map[string]any
	}
	getJSON(t, srv.URL+"/api/v2/details/"+itoa(det.ID), &detail)
	if len(detail.Blocks) != 2 {
		t.Fatalf("blocks %+v", detail)
	}
	for _, b := range detail.Blocks {
		if _, ok := b["data"]; ok {
			t.Fatalf("block list must not include data: %v", b)
		}
	}
	var blk Node
	getJSON(t, srv.URL+"/api/v2/details/"+itoa(det.ID)+"/blocks/triggers", &blk)
	if string(blk.Data) != `{"hs300":17.2}` || blk.SchemaVersion != 2 {
		t.Fatalf("block %+v", blk)
	}
	// 类型不符、不存在、写方法 都不行
	for _, u := range []string{"/api/v2/details/" + itoa(list.ID), "/api/v2/lists/" + itoa(det.ID) + "/details", "/api/v2/modules/nope/lists", "/api/v2/details/abc"} {
		if res := getJSON(t, srv.URL+u, nil); res.StatusCode != 404 && res.StatusCode != 400 {
			t.Fatalf("%s -> %d", u, res.StatusCode)
		}
	}
	res2, _ := http.Post(srv.URL+"/api/v2/modules", "application/json", strings.NewReader("{}"))
	if res2.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST should be 405, got %d", res2.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/v2/modules", nil)
	res3, _ := http.DefaultClient.Do(req)
	if res3.StatusCode != 204 || res3.Header.Get("Access-Control-Allow-Methods") != "GET" {
		t.Fatalf("preflight %d %v", res3.StatusCode, res3.Header)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestPagesOnSameHostAreSandboxed(t *testing.T) {
	srv, c := contentServer(t, contentConfig{})
	_, _ = c.PutPage("finance/drawdown", map[string]string{"index.html": "<h1>hi</h1>", "app.js": "1"})
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, _ := noRedirect.Get(srv.URL + "/pages/finance/drawdown")
	if res.StatusCode != 301 || res.Header.Get("Location") != "drawdown/" {
		t.Fatalf("trailing slash redirect %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	res, _ = http.Get(srv.URL + "/pages/finance/drawdown/")
	body, _ := io.ReadAll(res.Body)
	csp := res.Header.Get("Content-Security-Policy")
	if string(body) != "<h1>hi</h1>" || !strings.Contains(csp, "sandbox") || !strings.Contains(csp, "frame-ancestors 'none'") ||
		!strings.Contains(csp, "connect-src https://lssl.work") || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("page %q csp=%q", body, csp)
	}
	res, _ = http.Get(srv.URL + "/pages/finance/drawdown/app.js")
	if !strings.Contains(res.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("js content type %q", res.Header.Get("Content-Type"))
	}
	if res, _ := http.Get(srv.URL + "/pages/finance/drawdown/missing.js"); res.StatusCode != 404 {
		t.Fatalf("missing file %d", res.StatusCode)
	}
}

func TestPagesHostIsolation(t *testing.T) {
	srv, c := contentServer(t, contentConfig{PagesHost: "pages.example", APIOrigin: "https://api.example"})
	_, _ = c.PutPage("news", map[string]string{"index.html": "ok"})
	do := func(method, host, p string, hdr map[string]string) *http.Response {
		req, _ := http.NewRequest(method, srv.URL+p, bytes.NewReader(nil))
		req.Host = host
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := do("GET", "pages.example", "/news/", nil)
	body, _ := io.ReadAll(res.Body)
	csp := res.Header.Get("Content-Security-Policy")
	if string(body) != "ok" || strings.Contains(csp, "sandbox") || !strings.Contains(csp, "connect-src https://api.example") {
		t.Fatalf("pages host %q %q", body, csp)
	}
	// 页面域名下不暴露 MCP、token 管理和读接口以外的任何东西
	for _, p := range []string{"/mcp", "/api/tokens", "/api/v2/modules"} {
		if r := do("GET", "pages.example", p, map[string]string{"Authorization": "Bearer secret"}); r.StatusCode != 404 {
			t.Fatalf("%s on pages host -> %d", p, r.StatusCode)
		}
	}
	if r := do("POST", "pages.example", "/mcp", map[string]string{"Authorization": "Bearer secret"}); r.StatusCode != 405 {
		t.Fatalf("POST on pages host -> %d", r.StatusCode)
	}
	// 主域名访问 /pages/ 重定向到独立域名
	if r := do("GET", "main.example", "/pages/news/", nil); r.StatusCode != 302 || r.Header.Get("Location") != "https://pages.example/news/" {
		t.Fatalf("redirect %d %s", r.StatusCode, r.Header.Get("Location"))
	}
}

func TestWriteNeedsBearerNotCookie(t *testing.T) {
	srv, _ := contentServer(t, contentConfig{})
	req, _ := http.NewRequest("POST", srv.URL+"/mcp", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: "token", Value: "secret"})
	req.AddCookie(&http.Cookie{Name: "Authorization", Value: "Bearer secret"})
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cookie must not authorize: %d", res.StatusCode)
	}
}

func TestContentMCPTools(t *testing.T) {
	c := newMemContent()
	server := newMCPServer(newMemBoard())
	addContentTools(server, c)
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		txt := r.Content[0].(*mcp.TextContent).Text
		if r.IsError {
			t.Fatalf("%s error: %s", name, txt)
		}
		var out map[string]any
		_ = json.Unmarshal([]byte(txt), &out)
		return out
	}
	mod := call("put_node", map[string]any{"type": "module", "slug": "news", "title": "新闻"})
	lst := call("put_node", map[string]any{"parent_id": mod["id"], "type": "list", "slug": "days"})
	det := call("put_node", map[string]any{"parent_id": lst["id"], "type": "detail", "slug": "2026-10-16", "data": map[string]any{"n": 3}})
	call("put_node", map[string]any{"parent_id": det["id"], "type": "block", "slug": "top", "data": []any{"a", "b"}, "schema_version": 2})
	if p := call("list_nodes", map[string]any{"parent_id": det["id"], "type": "block"}); p["total"].(float64) != 1 {
		t.Fatalf("list_nodes %v", p)
	}
	if u := call("update_node", map[string]any{"id": det["id"], "title": "周五"}); u["title"] != "周五" {
		t.Fatalf("update %v", u)
	}
	if d := call("delete_node", map[string]any{"id": lst["id"]}); d["deleted"].(float64) != 3 {
		t.Fatalf("delete %v", d)
	}
	call("put_page", map[string]any{"path": "news/today", "files": map[string]any{"index.html": "a"}})
	call("put_page", map[string]any{"path": "news/today", "files": map[string]any{"index.html": "b"}})
	call("rollback_page", map[string]any{"path": "news/today", "version": 1})
	vs := call("list_page_versions", map[string]any{"path": "news/today"})["versions"].([]any)
	if len(vs) != 2 || vs[1].(map[string]any)["current"] != true {
		t.Fatalf("versions %v", vs)
	}
}
