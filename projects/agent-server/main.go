package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func writeAPI(w http.ResponseWriter, data any, errStr string) {
	w.Header().Set("Content-Type", "application/json")
	if errStr != "" {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": errStr, "data": data})
}

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = "0.0.0.0:80"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("hello %s %s", r.Method, r.URL.Path)
		writeAPI(w, map[string]string{
			"service": "agent-server",
			"msg":     "hello from agent-server",
		}, "")
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeAPI(w, "ok", "")
	})
	log.Println("agent-server", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
