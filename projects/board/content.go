package main

// 通用树形内容存储：模块(module) → 详情(detail) → 区块(block)。
// 旧版有 list 层（module → list → detail），已删除；启动时 flattenLists 迁移历史数据（幂等）。
// 每个节点的 data 是任意 JSON，带 schema_version，页面按数据结构自适应展示。
// 页面（Agent 上传的 HTML/JS/CSS）按路径存多个版本，current 指针决定线上版本，可回滚。

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	NodeModule = "module"
	NodeDetail = "detail"
	NodeBlock  = "block"
)

// parentType 规定每种节点的父节点类型；module 挂在根（parent_id=0）。
var parentType = map[string]string{
	NodeModule: "",
	NodeDetail: NodeModule,
	NodeBlock:  NodeDetail,
}

var (
	errNotFound = errors.New("not found")
	errConflict = errors.New("conflict")
)

// reservedModuleSlugs：会和 /api/v2/{module} 路由撞名的模块名。
var reservedModuleSlugs = map[string]bool{"modules": true}

func checkModuleSlug(typ, slug string) error {
	if typ == NodeModule && reservedModuleSlugs[slug] {
		return fmt.Errorf("模块名 %q 是保留字", slug)
	}
	return nil
}

type Node struct {
	ID            int64           `json:"id"`
	ParentID      int64           `json:"parent_id"`
	Type          string          `json:"type"`
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	Data          json.RawMessage `json:"data,omitempty"`
	SchemaVersion int             `json:"schema_version"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// brief 去掉 data，用在详情的区块清单里。
func (n Node) brief() Node { n.Data = nil; return n }

type NodeInput struct {
	ParentID      int64
	Type          string
	Slug          string
	Title         string
	Data          json.RawMessage
	SchemaVersion int
}

type NodePage struct {
	Items    []Node `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int    `json:"total"`
}

type PageVersion struct {
	Path      string   `json:"path"`
	Version   int      `json:"version"`
	Files     []string `json:"files"`
	Current   bool     `json:"current"`
	CreatedAt string   `json:"created_at"`
}

type ContentStore interface {
	PutNode(in NodeInput) (Node, error)
	UpdateNode(id int64, title *string, data *json.RawMessage, schemaVersion *int) (Node, error)
	DeleteNode(id int64) (int, error)
	GetNode(id int64) (Node, error)
	FindChild(parentID int64, typ, slug string) (Node, error)
	ListChildren(parentID int64, typ string, page, size int) (NodePage, error)
	// MoveNode 把节点改挂到 parentID 下（只改 parent_id，不校验类型）；新父下已有同 slug 时 errConflict。
	MoveNode(id, parentID int64) error

	PutPage(p string, files map[string]string) (PageVersion, error)
	ListPageVersions(p string) ([]PageVersion, error)
	RollbackPage(p string, version int) (PageVersion, error)
	// PageFiles 返回 current 版本的全部文件；没有该路径时 errNotFound。
	PageFiles(p string) (map[string]string, error)
	// ListPagePaths 返回所有有当前版本的页面路径（已排序），供导航页使用。
	ListPagePaths() ([]string, error)
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

func cleanSlug(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !slugRe.MatchString(s) {
		return "", fmt.Errorf("slug 只能是小写字母、数字、. _ -，1~128 位，且以字母或数字开头: %q", s)
	}
	return s, nil
}

func cleanNodeData(raw json.RawMessage) (json.RawMessage, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("data 不是合法 JSON")
	}
	if len(raw) > 4<<20 {
		return nil, fmt.Errorf("data 超过 4MB")
	}
	return raw, nil
}

func cleanSchemaVersion(v int) (int, error) {
	if v == 0 {
		return 1, nil
	}
	if v < 0 {
		return 0, fmt.Errorf("schema_version 必须 >= 1")
	}
	return v, nil
}

// checkParent 校验类型层级；parent 为 nil 表示根。
func checkParent(typ string, parent *Node) error {
	want, ok := parentType[typ]
	if !ok {
		return fmt.Errorf("type 只能是 module/detail/block: %q", typ)
	}
	if want == "" {
		if parent != nil {
			return fmt.Errorf("module 必须挂在根（parent_id=0）")
		}
		return nil
	}
	if parent == nil {
		return fmt.Errorf("%s 必须有父节点（%s）", typ, want)
	}
	if parent.Type != want {
		return fmt.Errorf("%s 的父节点必须是 %s，实际是 %s", typ, want, parent.Type)
	}
	return nil
}

func nodePageArgs(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

var (
	pageSegRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	pageFileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

const (
	maxPageDepth = 4
	maxPageFiles = 64
	maxPageBytes = 4 << 20
)

// cleanPagePath：1~4 段，每段小写字母数字 _ -，例如 finance/drawdown。
func cleanPagePath(p string) (string, error) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	segs := strings.Split(p, "/")
	if p == "" || len(segs) > maxPageDepth {
		return "", fmt.Errorf("页面路径需 1~%d 段: %q", maxPageDepth, p)
	}
	for _, s := range segs {
		if !pageSegRe.MatchString(s) {
			return "", fmt.Errorf("页面路径段只能是小写字母、数字、_ -: %q", s)
		}
	}
	return p, nil
}

// cleanPageFileName：允许子目录（如 assets/app.js），禁止 .. 与绝对路径。
func cleanPageFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("文件名非法: %q", name)
	}
	segs := strings.Split(name, "/")
	if len(segs) > 3 {
		return "", fmt.Errorf("文件目录最多 3 层: %q", name)
	}
	for _, s := range segs {
		if s == "." || s == ".." || !pageFileRe.MatchString(s) {
			return "", fmt.Errorf("文件名非法: %q", name)
		}
	}
	if path.Clean(name) != name {
		return "", fmt.Errorf("文件名非法: %q", name)
	}
	return name, nil
}

func cleanPageFiles(files map[string]string) (map[string]string, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("files 不能为空")
	}
	if len(files) > maxPageFiles {
		return nil, fmt.Errorf("文件数超过 %d", maxPageFiles)
	}
	out := make(map[string]string, len(files))
	total := 0
	for k, v := range files {
		n, err := cleanPageFileName(k)
		if err != nil {
			return nil, err
		}
		total += len(v)
		out[n] = v
	}
	if total > maxPageBytes {
		return nil, fmt.Errorf("页面总大小超过 %d 字节", maxPageBytes)
	}
	if _, ok := out["index.html"]; !ok {
		return nil, fmt.Errorf("必须包含 index.html")
	}
	return out, nil
}
