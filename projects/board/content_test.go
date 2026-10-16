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

// seedTree 建一棵 finance/r19/{triggers,funds}。
func seedTree(t *testing.T, c ContentStore) (mod, det Node) {
	mod = mustPut(t, c, NodeInput{Type: NodeModule, Slug: "finance", Title: "理财"})
	det = mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: "2026-10-16-r19", Data: json.RawMessage(`{"run_id":19}`)})
	mustPut(t, c, NodeInput{ParentID: det.ID, Type: NodeBlock, Slug: "triggers", Data: json.RawMessage(`{"hs300":17.2}`), SchemaVersion: 2})
	mustPut(t, c, NodeInput{ParentID: det.ID, Type: NodeBlock, Slug: "funds", Data: json.RawMessage(`[1,2]`)})
	return
}

func TestNodeHierarchyRules(t *testing.T) {
	c := newMemContent()
	mod, det := seedTree(t, c)
	bad := []NodeInput{
		{Type: "list", Slug: "x"}, // list 层已删除
		{ParentID: mod.ID, Type: "list", Slug: "x"},
		{ParentID: mod.ID, Type: NodeModule, Slug: "x"}, // module 不能有父
		{Type: NodeDetail, Slug: "x"},                   // detail 必须有父（module）
		{Type: NodeModule, Slug: "modules"},             // 保留模块名
		{ParentID: mod.ID, Type: NodeBlock, Slug: "x"},      // block 父必须是 detail
		{ParentID: det.ID, Type: NodeDetail, Slug: "x"},     // detail 父必须是 module
		{ParentID: det.ID, Type: "page", Slug: "x"},         // 未知类型
		{Type: NodeModule, Slug: "Bad Slug"},                // slug 非法
		{Type: NodeModule, Slug: "m", Data: []byte(`{bad`)}, // 非法 JSON
		{ParentID: 9999, Type: NodeDetail, Slug: "x"},       // 父不存在
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
	mod, det := seedTree(t, c)
	again := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: "2026-10-16-r19", Title: "运行记录"})
	if again.ID != det.ID || again.Title != "运行记录" {
		t.Fatalf("upsert should keep id: %+v", again)
	}
	n, err := c.DeleteNode(det.ID)
	if err != nil || n != 3 { // detail + 2 blocks
		t.Fatalf("delete count %d err %v", n, err)
	}
	if p, _ := c.ListChildren(mod.ID, NodeDetail, 1, 20); p.Total != 0 {
		t.Fatalf("details left: %+v", p)
	}
	if _, err := c.DeleteNode(det.ID); err != errNotFound {
		t.Fatalf("double delete: %v", err)
	}
	// 复活：同 parent+slug 再 put
	back := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: "2026-10-16-r19"})
	if back.ID != det.ID {
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
	mod, _ := seedTree(t, c)
	for i := 0; i < 25; i++ {
		mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: "d" + string(rune('a'+i))})
	}
	p, _ := c.ListChildren(mod.ID, NodeDetail, 2, 10)
	if p.Total != 26 || len(p.Items) != 10 || p.Page != 2 {
		t.Fatalf("page %+v", p)
	}
	p, _ = c.ListChildren(mod.ID, NodeDetail, 1, 500)
	if p.PageSize != 100 {
		t.Fatalf("size cap %d", p.PageSize)
	}
	if first, _ := c.ListChildren(mod.ID, NodeDetail, 1, 1); first.Items[0].Slug != "dy" {
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
	_, det := seedTree(t, c)
	var mods NodePage
	res := getJSON(t, srv.URL+"/api/v2/modules", &mods)
	if res.Header.Get("Access-Control-Allow-Origin") != "*" || mods.Total != 1 || mods.Items[0].Slug != "finance" {
		t.Fatalf("modules %+v %v", mods, res.Header)
	}
	var detail struct {
		Detail Node
		Blocks []map[string]any
	}
	getJSON(t, srv.URL+"/api/v2/finance/"+det.Slug, &detail)
	if len(detail.Blocks) != 2 {
		t.Fatalf("blocks %+v", detail)
	}
	for _, b := range detail.Blocks {
		if _, ok := b["data"]; ok {
			t.Fatalf("block list must not include data: %v", b)
		}
	}
	var blk Node
	getJSON(t, srv.URL+"/api/v2/finance/"+det.Slug+"/blocks/triggers", &blk)
	if string(blk.Data) != `{"hs300":17.2}` || blk.SchemaVersion != 2 {
		t.Fatalf("block %+v", blk)
	}
	// 旧的 list 层接口与按 id 的接口都已删除
	for _, u := range []string{"/api/v2/modules/finance/lists", "/api/v2/lists/1/details", "/api/v2/details/" + itoa(det.ID),
		"/api/v2/details/" + itoa(det.ID) + "/blocks/triggers"} {
		if res := getJSON(t, srv.URL+u, nil); res.StatusCode != 404 {
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
	det := call("put_node", map[string]any{"parent_id": mod["id"], "type": "detail", "slug": "2026-10-16", "data": map[string]any{"n": 3}})
	call("put_node", map[string]any{"parent_id": det["id"], "type": "block", "slug": "top", "data": []any{"a", "b"}, "schema_version": 2})
	if p := call("list_nodes", map[string]any{"parent_id": det["id"], "type": "block"}); p["total"].(float64) != 1 {
		t.Fatalf("list_nodes %v", p)
	}
	if u := call("update_node", map[string]any{"id": det["id"], "title": "周五"}); u["title"] != "周五" {
		t.Fatalf("update %v", u)
	}
	// 按名字写：module 填模块 slug，block 再加 detail 填详情 slug，不用查 id
	d2 := call("put_node", map[string]any{"module": "news", "type": "detail", "slug": "2026-10-17"})
	b2 := call("put_node", map[string]any{"module": "news", "detail": "2026-10-17", "type": "block", "slug": "top"})
	if d2["parent_id"] != mod["id"] || b2["parent_id"] != d2["id"] {
		t.Fatalf("named put %v %v", d2, b2)
	}
	if d := call("delete_node", map[string]any{"id": det["id"]}); d["deleted"].(float64) != 2 {
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

func TestDetailIncludeData(t *testing.T) {
	srv, c := contentServer(t, contentConfig{})
	_, det := seedTree(t, c)
	type resp struct {
		Blocks []Node `json:"blocks"`
	}
	for _, tc := range []struct {
		q    string
		data bool
	}{{"", false}, {"?include=data", true}, {"?include=other", false}} {
		var out resp
		if res := getJSON(t, srv.URL+"/api/v2/finance/"+det.Slug+tc.q, &out); res.StatusCode != 200 {
			t.Fatalf("%s -> %d", tc.q, res.StatusCode)
		}
		if len(out.Blocks) != 2 {
			t.Fatalf("%s blocks %d", tc.q, len(out.Blocks))
		}
		got := map[string]string{}
		for _, b := range out.Blocks {
			got[b.Slug] = string(b.Data)
		}
		if tc.data && (got["triggers"] != `{"hs300":17.2}` || got["funds"] != `[1,2]`) {
			t.Fatalf("include=data: %v", got)
		}
		if !tc.data && (got["triggers"] != "" || got["funds"] != "") {
			t.Fatalf("%s should omit data: %v", tc.q, got)
		}
	}
}

func TestTwoLevelNamedRoutes(t *testing.T) {
	srv, c := contentServer(t, contentConfig{})
	mod := mustPut(t, c, NodeInput{Type: NodeModule, Slug: "finance", Title: "理财"})
	det := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: "digest-30-evening", Data: json.RawMessage(`{"run_id":30}`)})
	mustPut(t, c, NodeInput{ParentID: det.ID, Type: NodeBlock, Slug: "conclusion", Data: json.RawMessage(`{"text":"不动"}`)})

	var list struct {
		Module  Node
		Details NodePage
	}
	if r := getJSON(t, srv.URL+"/api/v2/finance?size=5", &list); r.StatusCode != 200 || list.Module.ID != mod.ID || list.Details.Total != 1 || list.Details.PageSize != 5 {
		t.Fatalf("module details %d %+v", r.StatusCode, list)
	}
	if r := getJSON(t, srv.URL+"/api/v2/finance", nil); r.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("cors")
	}
	var one struct {
		Detail Node
		Blocks []Node
	}
	getJSON(t, srv.URL+"/api/v2/finance/digest-30-evening?include=data", &one)
	if one.Detail.ID != det.ID || len(one.Blocks) != 1 || string(one.Blocks[0].Data) != `{"text":"不动"}` {
		t.Fatalf("detail %+v", one)
	}
	var b Node
	if r := getJSON(t, srv.URL+"/api/v2/finance/digest-30-evening/blocks/conclusion", &b); r.StatusCode != 200 || b.Slug != "conclusion" {
		t.Fatalf("block %d %+v", r.StatusCode, b)
	}
	// 字面路由不被 {module} 抢走
	var mods NodePage
	if getJSON(t, srv.URL+"/api/v2/modules", &mods); mods.Total != 1 {
		t.Fatalf("modules %+v", mods)
	}
	for _, u := range []string{"/api/v2/nope", "/api/v2/finance/nope", "/api/v2/finance/digest-30-evening/blocks/nope",
		"/api/v2/finance/digest-30-evening/blocks", "/api/v2/a/b/c/d/e"} {
		if r := getJSON(t, srv.URL+u, nil); r.StatusCode != 404 {
			t.Fatalf("%s -> %d (want 404, not 405)", u, r.StatusCode)
		}
	}
}

// legacyNode 直接往内存库塞一条旧版 list 节点（新代码的 PutNode 已不接受 list）。
func legacyNode(c *memContent, parent int64, slug string) Node {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	n := &memNode{Node: Node{ID: c.nextID, ParentID: parent, Type: legacyList, Slug: slug, Data: json.RawMessage(`{}`), SchemaVersion: 1}, seq: c.nextID}
	c.nodes[n.ID] = n
	return n.Node
}

func TestFlattenListsMigration(t *testing.T) {
	c := newMemContent()
	mod := mustPut(t, c, NodeInput{Type: NodeModule, Slug: "news"})
	lst := legacyNode(c, mod.ID, "daily")
	var ids []int64
	for _, s := range []string{"2026-10-15", "2026-10-16"} {
		d := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: s})
		mustPut(t, c, NodeInput{ParentID: d.ID, Type: NodeBlock, Slug: "top"})
		if err := c.MoveNode(d.ID, lst.ID); err != nil { // 造旧数据：detail 挂在 list 下
			t.Fatal(err)
		}
		ids = append(ids, d.ID)
	}
	// 第二个模块：有同名冲突，list 应保留
	mod2 := mustPut(t, c, NodeInput{Type: NodeModule, Slug: "finance"})
	lst2 := legacyNode(c, mod2.ID, "digest")
	clash := mustPut(t, c, NodeInput{ParentID: mod2.ID, Type: NodeDetail, Slug: "x"})
	_ = clash
	d3 := mustPut(t, c, NodeInput{ParentID: mod.ID, Type: NodeDetail, Slug: "x"})
	if err := c.MoveNode(d3.ID, lst2.ID); err != nil {
		t.Fatal(err)
	}
	moved, skipped, err := flattenLists(c)
	if err != nil || moved != 2 || skipped != 1 {
		t.Fatalf("moved %d skipped %d err %v", moved, skipped, err)
	}
	if d, _ := c.GetNode(ids[0]); d.ParentID != mod.ID {
		t.Fatalf("not moved %+v", d)
	}
	if p, _ := c.ListChildren(ids[0], NodeBlock, 1, 10); p.Total != 1 {
		t.Fatal("blocks must follow detail")
	}
	if _, err := c.GetNode(lst.ID); err != errNotFound {
		t.Fatal("empty list should be soft-deleted")
	}
	if _, err := c.GetNode(lst2.ID); err != nil {
		t.Fatal("list with conflicts must stay")
	}
	if moved, skipped, _ := flattenLists(c); moved != 0 || skipped != 1 {
		t.Fatalf("not idempotent: %d %d", moved, skipped)
	}
}
