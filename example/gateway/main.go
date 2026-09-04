package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"minikitex/generic"
	"minikitex/idl"
)

var paramPat = regexp.MustCompile(`:([A-Za-z]+)`)

type backend struct {
	name string
	addr string
	spec *idl.Spec
}

type gw struct {
	mu   sync.Mutex
	clis map[string]*generic.Client
}

func main() {
	addr := getenv("LISTEN", "127.0.0.1:8080")
	backends, err := loadBackends()
	if err != nil {
		log.Fatal(err)
	}
	g := &gw{clis: map[string]*generic.Client{}}
	mux := http.NewServeMux()
	for _, b := range backends {
		if err := g.ensure(b); err != nil {
			log.Fatal(b.name, err)
		}
		for _, m := range b.spec.Methods {
			if m.URI == "" {
				continue
			}
			httpMethod := m.HTTPMethod
			if httpMethod == "" {
				httpMethod = "POST"
			}
			pattern := httpMethod + " " + paramPat.ReplaceAllString(m.URI, "{$1}")
			bb, mm := b, m
			mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				g.handle(w, r, bb, mm)
			})
			log.Println(pattern, "->", b.name+"."+m.Name)
		}
	}
	log.Println("agw sidecar", addr)
	log.Fatal(http.ListenAndServe(addr, cors(mux)))
}

func loadBackends() ([]*backend, error) {
	if one := os.Getenv("IDL"); one != "" {
		spec, err := idl.ParseFile(one)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(one), filepath.Ext(one))
		return []*backend{{name: name, addr: addrFor(name), spec: spec}}, nil
	}
	dir := getenv("IDL_DIR", "idl")
	matches, err := filepath.Glob(filepath.Join(dir, "*.thrift"))
	if err != nil {
		return nil, err
	}
	var out []*backend
	for _, path := range matches {
		spec, err := idl.ParseFile(path)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		hasHTTP := false
		for _, m := range spec.Methods {
			if m.URI != "" {
				hasHTTP = true
				break
			}
		}
		if !hasHTTP {
			continue
		}
		out = append(out, &backend{name: name, addr: addrFor(name), spec: spec})
	}
	return out, nil
}

func addrFor(name string) string {
	env := strings.ToUpper(name) + "_ADDR"
	def := "127.0.0.1:8889"
	if name == "user" {
		def = "127.0.0.1:8888"
	}
	if name == "platform" {
		def = "127.0.0.1:8887"
	}
	return getenv(env, def)
}

func apiReq(spec *idl.Spec, reqName string) bool {
	st, err := spec.Struct(reqName)
	if err != nil {
		return false
	}
	_, ok := st.FieldByName("token")
	return ok
}

func (g *gw) handle(w http.ResponseWriter, r *http.Request, b *backend, m idl.Method) {
	wrapped := apiReq(b.spec, m.Req)
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeErr(w, 400, err)
		return
	}
	req := any(body)
	if wrapped {
		raw, _ := json.Marshal(body)
		if string(raw) == "null" || len(body) == 0 {
			raw = []byte("{}")
		}
		req = map[string]any{
			"name":  r.PathValue("name"),
			"id":    r.PathValue("id"),
			"body":  string(raw),
			"token": strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")),
			"query": r.URL.RawQuery,
		}
	}
	if m.Name == "WatchBuild" {
		g.ssePoll(w, r, b, "GetBuild", strID(r.PathValue("id")))
		return
	}
	resp, err := g.rpc(r, b, m.Name, req)
	if err != nil {
		writeErr(w, rpcStatus(err), fmt.Errorf("%s", rpcMessage(err)))
		return
	}
	if m.Name == "CreateDeploy" {
		data := unwrap(resp)
		id := jsonID(data)
		if id == "" {
			writeEnvelope(w, wrapped, resp)
			return
		}
		g.ssePoll(w, r, b, "GetDeploy", id)
		return
	}
	writeEnvelope(w, wrapped, resp)
}

func (g *gw) rpc(r *http.Request, b *backend, method string, req any) (any, error) {
	var resp any
	err := g.call(b, func(cli *generic.Client) error {
		var e error
		resp, e = cli.Call(r.Context(), method, req)
		return e
	})
	return resp, err
}

func (g *gw) ssePoll(w http.ResponseWriter, r *http.Request, b *backend, method, id string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, fmt.Errorf("stream not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	prev := ""
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	send := func(event string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		fl.Flush()
	}
	for {
		req := map[string]any{
			"id":    id,
			"body":  "{}",
			"token": strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")),
		}
		resp, err := g.rpc(r, b, method, req)
		if err != nil {
			send("done", map[string]any{"status": "fail", "error": rpcMessage(err)})
			return
		}
		data, _ := unwrap(resp).(map[string]any)
		if data == nil {
			data = map[string]any{}
		}
		logText, _ := data["log"].(string)
		if logText != prev {
			send("log", map[string]string{"text": strings.TrimPrefix(logText, prev)})
			prev = logText
		}
		st, _ := data["status"].(string)
		if st != "" && st != "running" {
			send("done", data)
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
	}
}

func unwrap(resp any) any {
	m, ok := resp.(map[string]any)
	if !ok {
		return resp
	}
	body, ok := m["body"].(string)
	if !ok {
		return resp
	}
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return resp
	}
	return data
}

func writeEnvelope(w http.ResponseWriter, wrapped bool, resp any) {
	if wrapped {
		writeJSON(w, map[string]any{"error": "", "data": unwrap(resp)})
		return
	}
	writeJSON(w, resp)
}

func rpcStatus(err error) int {
	msg := err.Error()
	_, after, ok := strings.Cut(msg, "HTTP ")
	if !ok {
		return 502
	}
	n := 0
	fmt.Sscanf(after, "%d", &n)
	if n >= 400 && n < 600 {
		return n
	}
	return 502
}

func rpcMessage(err error) string {
	msg := err.Error()
	_, after, ok := strings.Cut(msg, "HTTP ")
	if !ok {
		return msg
	}
	if i := strings.Index(after, ": "); i >= 0 {
		return after[i+2:]
	}
	return msg
}

func strID(s string) string { return s }

func jsonID(v any) string {
	m, _ := v.(map[string]any)
	if m == nil {
		return ""
	}
	switch id := m["id"].(type) {
	case string:
		return id
	case float64:
		return strconv.FormatInt(int64(id), 10)
	case json.Number:
		return id.String()
	case int64:
		return strconv.FormatInt(id, 10)
	default:
		if id == nil {
			return ""
		}
		return fmt.Sprint(id)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *gw) call(b *backend, fn func(*generic.Client) error) error {
	if err := g.ensure(b); err != nil {
		return err
	}
	g.mu.Lock()
	cli := g.clis[b.addr]
	g.mu.Unlock()
	err := fn(cli)
	if err == nil {
		return nil
	}
	ncli, dialErr := generic.Dial(b.addr, b.spec)
	if dialErr != nil {
		return err
	}
	g.mu.Lock()
	old := g.clis[b.addr]
	g.clis[b.addr] = ncli
	g.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return fn(ncli)
}

func (g *gw) ensure(b *backend) error {
	g.mu.Lock()
	if g.clis[b.addr] != nil {
		g.mu.Unlock()
		return nil
	}
	g.mu.Unlock()
	cli, err := dial(b.addr, b.spec)
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.clis[b.addr] = cli
	g.mu.Unlock()
	return nil
}

func dial(addr string, spec *idl.Spec) (*generic.Client, error) {
	var last error
	for i := 0; i < 50; i++ {
		cli, err := generic.Dial(addr, spec)
		if err == nil {
			return cli, nil
		}
		last = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, last
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
