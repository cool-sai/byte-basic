package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"minikitex/generic"
	"minikitex/idl"
)

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
			pattern := httpMethod + " " + m.URI
			bb, rpcMethod := b, m.Name
			mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				g.handle(w, r, bb, rpcMethod)
			})
			log.Println(pattern, "->", b.name+"."+rpcMethod)
		}
	}
	log.Println("gateway", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
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
	return getenv(env, def)
}

func (g *gw) handle(w http.ResponseWriter, r *http.Request, b *backend, method string) {
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeErr(w, 400, err)
		return
	}
	var resp any
	err := g.call(b, func(cli *generic.Client) error {
		var e error
		resp, e = cli.Call(r.Context(), method, body)
		return e
	})
	if err != nil {
		writeErr(w, 502, err)
		return
	}
	writeJSON(w, resp)
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
