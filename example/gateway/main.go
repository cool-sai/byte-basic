package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"minikitex/generic"
	"minikitex/idl"
)

type gw struct {
	addr string
	spec *idl.Spec
	mu   sync.Mutex
	cli  *generic.Client
}

func main() {
	addr := getenv("LISTEN", "127.0.0.1:8080")
	orderAddr := getenv("ORDER_ADDR", "127.0.0.1:8889")
	spec, err := idl.ParseFile(getenv("IDL", "idl/order.thrift"))
	if err != nil {
		log.Fatal(err)
	}
	cli, err := dialOrder(orderAddr, spec)
	if err != nil {
		log.Fatal(err)
	}
	g := &gw{addr: orderAddr, spec: spec, cli: cli}

	mux := http.NewServeMux()
	for _, m := range spec.Methods {
		if m.URI == "" {
			continue
		}
		httpMethod := m.HTTPMethod
		if httpMethod == "" {
			httpMethod = "POST"
		}
		pattern := httpMethod + " " + m.URI
		rpcMethod := m.Name
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			g.handle(w, r, rpcMethod)
		})
		log.Println(pattern, "->", rpcMethod)
	}

	log.Println("gateway", addr, "->", orderAddr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (g *gw) handle(w http.ResponseWriter, r *http.Request, method string) {
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeErr(w, 400, err)
		return
	}
	var resp any
	err := g.rpc(func(cli *generic.Client) error {
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

func (g *gw) rpc(fn func(*generic.Client) error) error {
	g.mu.Lock()
	cli := g.cli
	g.mu.Unlock()
	err := fn(cli)
	if err == nil {
		return nil
	}
	ncli, dialErr := generic.Dial(g.addr, g.spec)
	if dialErr != nil {
		return err
	}
	g.mu.Lock()
	old := g.cli
	g.cli = ncli
	g.mu.Unlock()
	_ = old.Close()
	return fn(ncli)
}

func dialOrder(addr string, spec *idl.Spec) (*generic.Client, error) {
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
