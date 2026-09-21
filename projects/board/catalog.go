package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type toolDoc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Fields      []toolField `json:"fields"`
}

func catalogFromMCP(server *mcp.Server) ([]toolDoc, error) {
	tools, err := listServerTools(server)
	if err != nil {
		return nil, err
	}
	out := make([]toolDoc, 0, len(tools))
	for _, t := range tools {
		out = append(out, toolDoc{
			Name:        t.Name,
			Description: t.Description,
			Fields:      schemaFields(t.InputSchema),
		})
	}
	return out, nil
}

func listServerTools(server *mcp.Server) ([]*mcp.Tool, error) {
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer ss.Close()
	cl := mcp.NewClient(&mcp.Implementation{Name: "board-catalog", Version: "v0.1.0"}, nil)
	cs, err := cl.Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res.Tools, nil
}

func schemaFields(schema any) []toolField {
	var m map[string]any
	b, err := json.Marshal(schema)
	if err != nil || json.Unmarshal(b, &m) != nil {
		return nil
	}
	props, _ := m["properties"].(map[string]any)
	req := map[string]bool{}
	if raw, ok := m["required"].([]any); ok {
		for _, x := range raw {
			if s, ok := x.(string); ok {
				req[s] = true
			}
		}
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]toolField, 0, len(names))
	for _, name := range names {
		p, _ := props[name].(map[string]any)
		out = append(out, toolField{
			Name:        name,
			Type:        schemaType(p["type"]),
			Required:    req[name],
			Description: schemaString(p["description"]),
		})
	}
	return out
}

func schemaType(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var parts []string
		for _, x := range t {
			s, ok := x.(string)
			if !ok || s == "null" {
				continue
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, "|")
	default:
		return "any"
	}
}

func schemaString(v any) string {
	s, _ := v.(string)
	return s
}
