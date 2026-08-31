package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"minikitex/discovery"
	"minikitex/gen/user"
)

type Handler struct {
	instance string
}

func (h Handler) GetUser(_ context.Context, req *user.GetUserReq) (*user.GetUserResp, error) {
	names := map[int64]string{1: "alice", 2: "bob"}
	name, ok := names[req.ID]
	if !ok {
		return nil, fmt.Errorf("user %d not found", req.ID)
	}
	log.Printf("[%s] GetUser %d -> %s", h.instance, req.ID, name)
	return &user.GetUserResp{ID: req.ID, Name: name}, nil
}

func (Handler) Ping(context.Context, *user.PingReq) (*user.PingResp, error) {
	return &user.PingResp{Msg: "pong"}, nil
}

func main() {
	instance := os.Getenv("INSTANCE")
	if instance == "" {
		instance, _ = os.Hostname()
	}
	addr := getenv("LISTEN", "127.0.0.1:8888")
	if os.Getenv("REGISTRY") != "" {
		go announce(instance)
	}
	log.Println("user server", instance, "listening", addr)
	log.Fatal(user.NewServer(Handler{instance: instance}).ListenAndServe(addr))
}

func announce(instance string) {
	base := os.Getenv("REGISTRY")
	name := getenv("SERVICE_NAME", "user")
	advertise := os.Getenv("ADVERTISE")
	if advertise == "" {
		advertise = instance + ":8888"
	}
	for {
		if err := discovery.Register(base, name, advertise); err != nil {
			log.Println("register:", err)
		}
		time.Sleep(2 * time.Second)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
