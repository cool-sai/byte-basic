package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type putNodeArgs struct {
	ParentID      int64  `json:"parent_id,omitempty" jsonschema:"父节点 id；module 填 0 或不填"`
	Type          string `json:"type" jsonschema:"module / list / detail / block；层级固定 module→list→detail→block"`
	Slug          string `json:"slug" jsonschema:"同一父节点下唯一的短名，小写字母数字 . _ -，如 finance、runs、2026-10-16-r19、triggers"`
	Title         string `json:"title,omitempty" jsonschema:"展示标题"`
	Data          any    `json:"data,omitempty" jsonschema:"任意 JSON；页面按结构自适应展示"`
	SchemaVersion int    `json:"schema_version,omitempty" jsonschema:"data 的结构版本，默认 1；改了结构就加 1"`
}

type updateNodeArgs struct {
	ID            int64   `json:"id" jsonschema:"节点 id"`
	Title         *string `json:"title,omitempty" jsonschema:"新标题"`
	Data          any     `json:"data,omitempty" jsonschema:"新的 data（整体替换），不传则不改"`
	SchemaVersion *int    `json:"schema_version,omitempty" jsonschema:"新的结构版本"`
}

type nodeIDArgs struct {
	ID int64 `json:"id" jsonschema:"节点 id"`
}

type listNodesArgs struct {
	ParentID int64  `json:"parent_id,omitempty" jsonschema:"父节点 id，0 表示根（列模块）"`
	Type     string `json:"type" jsonschema:"要列的子节点类型 module / list / detail / block"`
	Page     int    `json:"page,omitempty" jsonschema:"页码，从 1 开始"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

type putPageArgs struct {
	Path  string            `json:"path" jsonschema:"页面路径 1~4 段，如 finance/drawdown；访问地址 https://pages.lssl.work/<path>/"`
	Files map[string]string `json:"files" jsonschema:"文件名→文本内容，必须含 index.html，可有 app.js、style.css、assets/x.js；每次保存为新版本并立即上线"`
}

type pagePathArgs struct {
	Path string `json:"path" jsonschema:"页面路径"`
}

type rollbackPageArgs struct {
	Path    string `json:"path" jsonschema:"页面路径"`
	Version int    `json:"version" jsonschema:"要切回的版本号（见 list_page_versions）"`
}

const contentInstructions = `

内容树（v2）：module（模块，如 finance）→ list（列表，如 runs）→ detail（详情，如某次运行）→ block（区块，如 triggers）。每个节点的 data 是任意 JSON，带 schema_version。
写：put_node（按 parent_id+slug 幂等覆盖，返回 id）、update_node、delete_node（软删除整棵子树）、list_nodes、get_node。
读接口（公开、只读，页面 JS 用）：/board/api/v2/modules、/modules/{slug}/lists、/lists/{id}/details?page=&size=、/details/{id}（含区块清单，不含 data）、/details/{id}/blocks/{slug}。
页面：put_page 上传 HTML/JS/CSS（每次一个新版本并立即上线），list_page_versions 看历史，rollback_page 切回旧版本。页面只能 fetch 上面的读接口，不能调写接口、不能嵌 iframe、不能提交表单。`

func addContentTools(server *mcp.Server, c ContentStore) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "put_node",
		Description: "创建或覆盖内容树节点：同一 parent_id 下按 slug 幂等（已删除的会复活）",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in putNodeArgs) (*mcp.CallToolResult, any, error) {
		data, err := anyJSON(in.Data)
		if err != nil {
			return nil, nil, err
		}
		n, err := c.PutNode(NodeInput{ParentID: in.ParentID, Type: in.Type, Slug: in.Slug, Title: in.Title, Data: data, SchemaVersion: in.SchemaVersion})
		if err != nil {
			return nil, nil, err
		}
		return textJSON(n), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_node",
		Description: "按 id 修改节点的 title / data / schema_version，只传要改的字段",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in updateNodeArgs) (*mcp.CallToolResult, any, error) {
		var data *json.RawMessage
		if in.Data != nil {
			raw, err := anyJSON(in.Data)
			if err != nil {
				return nil, nil, err
			}
			data = &raw
		}
		n, err := c.UpdateNode(in.ID, in.Title, data, in.SchemaVersion)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(n), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_node",
		Description: "软删除节点及其全部子孙，返回删除数量",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in nodeIDArgs) (*mcp.CallToolResult, any, error) {
		cnt, err := c.DeleteNode(in.ID)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(map[string]any{"ok": true, "id": in.ID, "deleted": cnt}), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node",
		Description: "按 id 读取一个节点（含 data）",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in nodeIDArgs) (*mcp.CallToolResult, any, error) {
		n, err := c.GetNode(in.ID)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(n), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_nodes",
		Description: "分页列出某父节点下某类型的子节点，新到旧",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listNodesArgs) (*mcp.CallToolResult, any, error) {
		if _, ok := parentType[in.Type]; !ok {
			return nil, nil, fmt.Errorf("type 只能是 module/list/detail/block")
		}
		p, err := c.ListChildren(in.ParentID, in.Type, in.Page, in.PageSize)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(p), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "put_page",
		Description: "上传页面文件（必须含 index.html），保存为新版本并立即切为当前版本",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in putPageArgs) (*mcp.CallToolResult, any, error) {
		v, err := c.PutPage(in.Path, in.Files)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(v), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_page_versions",
		Description: "列出页面的历史版本（新到旧），标出当前版本",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pagePathArgs) (*mcp.CallToolResult, any, error) {
		vs, err := c.ListPageVersions(in.Path)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(map[string]any{"versions": vs}), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "rollback_page",
		Description: "把页面的当前版本切回指定的历史版本",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in rollbackPageArgs) (*mcp.CallToolResult, any, error) {
		v, err := c.RollbackPage(in.Path, in.Version)
		if err != nil {
			return nil, nil, err
		}
		return textJSON(v), nil, nil
	})
}

func anyJSON(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("data 无法序列化: %w", err)
	}
	return b, nil
}
