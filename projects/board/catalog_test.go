package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolCatalogPageAndAPI(t *testing.T) {
	ts := httptest.NewServer(testMux(t, "secret"))
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/board/api/tools")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("api %d %s", res.StatusCode, b)
	}
	var doc struct {
		Tools []toolDoc `json:"tools"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Tools) != 9 {
		t.Fatalf("catalog %d", len(doc.Tools))
	}
	got := map[string]toolDoc{}
	for _, x := range doc.Tools {
		got[x.Name] = x
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/board/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("mcp %d %s", res.StatusCode, b)
	}
	var msg struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.Fatal(err)
	}
	if len(msg.Result.Tools) != len(doc.Tools) {
		t.Fatalf("mcp %d catalog %d", len(msg.Result.Tools), len(doc.Tools))
	}
	for _, m := range msg.Result.Tools {
		c, ok := got[m.Name]
		if !ok {
			t.Fatalf("catalog missing %s", m.Name)
		}
		if c.Description != m.Description {
			t.Fatalf("%s desc catalog %q mcp %q", m.Name, c.Description, m.Description)
		}
	}

	res, err = http.Get(ts.URL + "/board/tools/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), "MCP 工具") {
		t.Fatalf("page %d %s", res.StatusCode, b)
	}
}
