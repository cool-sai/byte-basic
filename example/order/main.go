package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"minikitex/gen/order"
	"minikitex/gen/user"
)

type peer struct {
	addr string
	cli  *user.Client
}

type Handler struct {
	users []*peer
	rr    atomic.Uint64

	mu     sync.Mutex
	orders map[int64]stored
	nextID int64
}

type stored struct {
	id     int64
	userID int64
	status string
}

func (h *Handler) pickUser() *peer {
	n := h.rr.Add(1)
	return h.users[(n-1)%uint64(len(h.users))]
}

func (h *Handler) GetOrder(ctx context.Context, req *order.GetOrderReq) (*order.GetOrderResp, error) {
	h.mu.Lock()
	o, ok := h.orders[req.ID]
	h.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("order %d not found", req.ID)
	}

	p := h.pickUser()
	log.Printf("GetOrder %d -> RPC user.GetUser(%d) via %s", o.id, o.userID, p.addr)
	u, err := p.cli.GetUser(ctx, &user.GetUserReq{ID: o.userID})
	if err != nil {
		return nil, err
	}
	return &order.GetOrderResp{
		ID:       o.id,
		UserId:   o.userID,
		Status:   o.status,
		UserName: u.Name,
	}, nil
}

func (h *Handler) CreateOrder(_ context.Context, req *order.CreateOrderReq) (*order.CreateOrderResp, error) {
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	h.orders[id] = stored{id: id, userID: req.UserId, status: "created"}
	h.mu.Unlock()
	return &order.CreateOrderResp{ID: id, Status: "created"}, nil
}

func main() {
	addrs := splitAddrs(getenv("USER_ADDR", "127.0.0.1:8888"))
	var peers []*peer
	for _, addr := range addrs {
		cli, err := dialUser(addr)
		if err != nil {
			log.Fatal("dial user ", addr, ": ", err)
		}
		defer cli.Close()
		peers = append(peers, &peer{addr: addr, cli: cli})
	}

	h := &Handler{
		users: peers,
		orders: map[int64]stored{
			1001: {id: 1001, userID: 1, status: "paid"},
		},
		nextID: 2000,
	}
	addr := getenv("LISTEN", "127.0.0.1:8889")
	log.Println("order server listening", addr, "users", addrs)
	log.Fatal(order.NewServer(h).ListenAndServe(addr))
}

func dialUser(addr string) (*user.Client, error) {
	var last error
	for i := 0; i < 50; i++ {
		cli, err := user.NewClient(addr)
		if err == nil {
			return cli, nil
		}
		last = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, last
}

func splitAddrs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
