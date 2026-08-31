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

	"minikitex/discovery"
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
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.users) == 0 {
		return nil
	}
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
	if p == nil {
		return nil, fmt.Errorf("no user instances")
	}
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

func (h *Handler) applyAddrs(addrs []string) {
	h.mu.Lock()
	have := map[string]*peer{}
	for _, p := range h.users {
		have[p.addr] = p
	}
	h.mu.Unlock()

	var next []*peer
	used := map[string]bool{}
	for _, addr := range addrs {
		used[addr] = true
		if p, ok := have[addr]; ok {
			next = append(next, p)
			continue
		}
		cli, err := dialUser(addr)
		if err != nil {
			log.Printf("dial user %s: %v", addr, err)
			continue
		}
		log.Println("discovered", addr)
		next = append(next, &peer{addr: addr, cli: cli})
	}
	h.mu.Lock()
	old := h.users
	h.users = next
	h.mu.Unlock()
	for _, p := range old {
		if !used[p.addr] {
			log.Println("dropped", p.addr)
			_ = p.cli.Close()
		}
	}
}

func main() {
	h := &Handler{
		orders: map[int64]stored{
			1001: {id: 1001, userID: 1, status: "paid"},
		},
		nextID: 2000,
	}

	if os.Getenv("REGISTRY") != "" {
		if err := waitAndWatch(h); err != nil {
			log.Fatal(err)
		}
	} else {
		addrs := splitAddrs(getenv("USER_ADDR", "127.0.0.1:8888"))
		h.applyAddrs(addrs)
		log.Println("static users", addrs)
	}

	addr := getenv("LISTEN", "127.0.0.1:8889")
	log.Println("order server listening", addr)
	log.Fatal(order.NewServer(h).ListenAndServe(addr))
}

func waitAndWatch(h *Handler) error {
	base := os.Getenv("REGISTRY")
	name := getenv("SERVICE_NAME", "user")
	var last error
	for i := 0; i < 50; i++ {
		addrs, err := discovery.Lookup(base, name)
		if err != nil {
			last = err
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if len(addrs) == 0 {
			last = fmt.Errorf("registry: no %s yet", name)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		h.applyAddrs(addrs)
		log.Println("users from registry", addrs)
		go func() {
			for {
				time.Sleep(2 * time.Second)
				addrs, err := discovery.Lookup(base, name)
				if err != nil {
					log.Println("lookup:", err)
					continue
				}
				h.applyAddrs(addrs)
			}
		}()
		return nil
	}
	return last
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
