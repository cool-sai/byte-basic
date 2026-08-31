package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"minikitex/discovery"
)

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8500"
	}
	log.Println("registry listening", addr)
	log.Fatal(http.ListenAndServe(addr, discovery.Handler(discovery.NewRegistry(8*time.Second))))
}
