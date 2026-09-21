package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"
)

type Kind struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Item struct {
	ID       int64          `json:"id"`
	KindID   int64          `json:"kind_id"`
	Kind     string         `json:"kind"`
	Content  string         `json:"content"`
	Type     string         `json:"type"`
	Date     string         `json:"date"`
	Extra    map[string]any `json:"extra"`
	Rendered string         `json:"rendered,omitempty"`
}

type KindHTML struct {
	ID     int64  `json:"id"`
	KindID int64  `json:"kind_id"`
	Kind   string `json:"kind"`
	HTML   string `json:"html"`
}

type KindCount struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Summary struct {
	Kinds     []KindCount `json:"kinds"`
	Total     int         `json:"total"`
	UpdatedAt string      `json:"updated_at"`
}

type ListPage struct {
	Items    []Item `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int    `json:"total"`
}

type Board interface {
	CreateKind(name string) (Kind, error)
	AddItem(kind, content, date, typ string, extra json.RawMessage) (Item, error)
	UpdateItem(id int64, kind, content, date, typ *string, extra *json.RawMessage) (Item, error)
	DeleteItem(id int64) error
	ListItems(kind string, page, pageSize int) (ListPage, error)
	Summary() (Summary, error)
	CreateKindHTML(kind, html string) (KindHTML, error)
	UpdateKindHTML(kind, html string) (KindHTML, error)
	DeleteKindHTML(kind string) error
	GetKindHTML(kind string) (KindHTML, error)
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("请填写名称")
	}
	if len([]rune(name)) > 64 {
		return "", fmt.Errorf("名称太长")
	}
	return name, nil
}

func cleanContent(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("请填写内容")
	}
	return s, nil
}

func parseDay(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return "", fmt.Errorf("日期写成 YYYY-MM-DD")
	}
	return t.Format("2006-01-02"), nil
}

func pageArgs(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func extraOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	return raw
}

func extraMap(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(extraOrEmpty(raw), &m)
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func mapExtra(m map[string]any) json.RawMessage {
	if m == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func parseKindHTML(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("请填写 html")
	}
	if len(s) > 64*1024 {
		return "", fmt.Errorf("html 太长")
	}
	if _, err := template.New("kind").Parse(s); err != nil {
		return "", fmt.Errorf("html 模板无效")
	}
	return s, nil
}

func contentData(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	return v
}

func applyHTML(items []Item, html string) {
	if html == "" {
		return
	}
	t, err := template.New("kind").Parse(html)
	if err != nil {
		return
	}
	for i := range items {
		var buf bytes.Buffer
		if err := t.Execute(&buf, contentData(items[i].Content)); err != nil {
			continue
		}
		items[i].Rendered = buf.String()
	}
}
