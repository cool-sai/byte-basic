package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createKindArgs struct {
	Name string `json:"name" jsonschema:"种类名称，如 算法、系统设计"`
}

type addItemArgs struct {
	Kind    string         `json:"kind" jsonschema:"种类名称"`
	Content string         `json:"content" jsonschema:"条目内容，可以是 JSON 文本；该种类若绑了 html 片段，按模板渲染"`
	Date    string         `json:"date,omitempty" jsonschema:"日期 YYYY-MM-DD，缺省今天"`
	Type    string         `json:"type,omitempty" jsonschema:"桶内分类，可空"`
	Extra   map[string]any `json:"extra,omitempty" jsonschema:"拓展字段"`
}

type updateItemArgs struct {
	ID      int64           `json:"id" jsonschema:"列表 id"`
	Kind    *string         `json:"kind,omitempty" jsonschema:"种类名称"`
	Content *string         `json:"content,omitempty" jsonschema:"内容"`
	Date    *string         `json:"date,omitempty" jsonschema:"日期 YYYY-MM-DD"`
	Type    *string         `json:"type,omitempty" jsonschema:"桶内分类"`
	Extra   *map[string]any `json:"extra,omitempty" jsonschema:"拓展字段"`
}

type deleteItemArgs struct {
	ID int64 `json:"id" jsonschema:"列表 id"`
}

type listItemsArgs struct {
	Kind     string `json:"kind" jsonschema:"种类名称"`
	Page     int    `json:"page,omitempty" jsonschema:"页码，从 1 开始"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

type kindHTMLArgs struct {
	Kind string `json:"kind" jsonschema:"种类名称"`
	HTML string `json:"html" jsonschema:"html 片段，Go template，根对象是 content 的 JSON，例如 {{.title}}"`
}

type kindNameArgs struct {
	Kind string `json:"kind" jsonschema:"种类名称"`
}

const mcpInstructions = `通用看板：按种类（kind）收条目（item），浏览器只读 https://lssl.work/board/ ，写数据只用这些工具。种类不限定场景，算法、待办、新闻、任意列表都可以。

三层：kind（桶）→ item（桶里一条）→ 可选的 kind html（每种一段 Go template，用来渲染该种类 item.content）。content 可以是纯文本或 JSON；绑了 html 时模板根对象是 content 解析后的 JSON，例如 {{.title}}、{{.url}}。没绑 html 时页面直接显示原文。

顺序：create_kind（同名幂等）→ 需要定制展示再 create_kind_html / get_kind_html / update_kind_html → add_item / list_items / update_item / delete_item。list_items 按种类分页、日期新到旧。例如链接列表可以把 content 写成 {"title":"...","url":"https://..."}，html 写成 <a href="{{.url}}">{{.title}}</a>。`

func newMCPServer(board Board) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "board", Version: "v0.1.0"}, &mcp.ServerOptions{
		Instructions: mcpInstructions + contentInstructions,
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_kind",
		Description: "创建种类（算法、系统设计、待办、每日新闻等）；同名返回已有记录",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in createKindArgs) (*mcp.CallToolResult, Kind, error) {
		k, err := board.CreateKind(in.Name)
		if err != nil {
			return nil, Kind{}, err
		}
		return textJSON(k), k, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_item",
		Description: "向某个种类插入一条列表",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in addItemArgs) (*mcp.CallToolResult, Item, error) {
		it, err := board.AddItem(in.Kind, in.Content, in.Date, in.Type, mapExtra(in.Extra))
		if err != nil {
			return nil, Item{}, err
		}
		return textJSON(it), it, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_item",
		Description: "按 id 修改列表字段，只传要改的字段",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in updateItemArgs) (*mcp.CallToolResult, Item, error) {
		var extra *json.RawMessage
		if in.Extra != nil {
			raw := mapExtra(*in.Extra)
			extra = &raw
		}
		it, err := board.UpdateItem(in.ID, in.Kind, in.Content, in.Date, in.Type, extra)
		if err != nil {
			return nil, Item{}, err
		}
		return textJSON(it), it, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_item",
		Description: "按 id 删除一条列表",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in deleteItemArgs) (*mcp.CallToolResult, any, error) {
		if err := board.DeleteItem(in.ID); err != nil {
			return nil, nil, err
		}
		return textJSON(map[string]any{"ok": true, "id": in.ID}), map[string]any{"ok": true, "id": in.ID}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_items",
		Description: "按种类分页查询列表，按日期从新到旧",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listItemsArgs) (*mcp.CallToolResult, ListPage, error) {
		page, err := board.ListItems(in.Kind, in.Page, in.PageSize)
		if err != nil {
			return nil, ListPage{}, err
		}
		return textJSON(page), page, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_kind_html",
		Description: "给种类绑定一段 html 片段，用来渲染该种类条目的 content；每种只能绑一段",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in kindHTMLArgs) (*mcp.CallToolResult, KindHTML, error) {
		h, err := board.CreateKindHTML(in.Kind, in.HTML)
		if err != nil {
			return nil, KindHTML{}, err
		}
		return textJSON(h), h, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_kind_html",
		Description: "按种类替换 html 片段",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in kindHTMLArgs) (*mcp.CallToolResult, KindHTML, error) {
		h, err := board.UpdateKindHTML(in.Kind, in.HTML)
		if err != nil {
			return nil, KindHTML{}, err
		}
		return textJSON(h), h, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_kind_html",
		Description: "按种类删除 html 片段",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in kindNameArgs) (*mcp.CallToolResult, any, error) {
		if err := board.DeleteKindHTML(in.Kind); err != nil {
			return nil, nil, err
		}
		return textJSON(map[string]any{"ok": true, "kind": in.Kind}), map[string]any{"ok": true, "kind": in.Kind}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_kind_html",
		Description: "查看种类当前绑定的 html 片段",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in kindNameArgs) (*mcp.CallToolResult, KindHTML, error) {
		h, err := board.GetKindHTML(in.Kind)
		if err != nil {
			return nil, KindHTML{}, err
		}
		return textJSON(h), h, nil
	})
	return server
}

func newMCPHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
		Stateless:    true,
	})
}

func textJSON(v any) *mcp.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "{}"}}}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

func bearerToken(r *http.Request) string {
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
	return strings.TrimSpace(got)
}

func matchAdmin(admin, got string) bool {
	return admin != "" && subtle.ConstantTimeCompare([]byte(got), []byte(admin)) == 1
}

func requireBearer(ok func(string) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok(bearerToken(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
