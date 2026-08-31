package minikitex_test

import (
	"bytes"
	"context"
	"net"
	"testing"

	"minikitex/gen/user"
	"minikitex/generic"
	"minikitex/idl"
)

type handler struct{}

func (handler) GetUser(_ context.Context, req *user.GetUserReq) (*user.GetUserResp, error) {
	return &user.GetUserResp{ID: req.ID, Name: "alice"}, nil
}

func (handler) Ping(context.Context, *user.PingReq) (*user.PingResp, error) {
	return &user.PingResp{Msg: "pong"}, nil
}

func start(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go user.NewServer(handler{}).Serve(ln)
	return ln.Addr().String()
}

func TestTypedEqualsGenericBytes(t *testing.T) {
	spec, err := idl.ParseFile("idl/user.idl")
	if err != nil {
		t.Fatal(err)
	}
	typed := user.EncodeGetUserReq(&user.GetUserReq{ID: 1})
	gen, err := generic.Encode(spec, "GetUserReq", map[string]any{"id": int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(typed, gen) {
		t.Fatalf("typed and generic must emit the same bytes\n%x\n%x", typed, gen)
	}
}

func TestTypedAndGenericHitSameServer(t *testing.T) {
	addr := start(t)
	ctx := context.Background()

	typed, err := user.NewClient(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer typed.Close()
	tr, err := typed.GetUser(ctx, &user.GetUserReq{ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Name != "alice" {
		t.Fatalf("typed=%+v", tr)
	}

	spec, err := idl.ParseFile("idl/user.idl")
	if err != nil {
		t.Fatal(err)
	}
	g, err := generic.Dial(addr, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	gr, err := g.Call(ctx, "GetUser", `{"id":1}`)
	if err != nil {
		t.Fatal(err)
	}
	m := gr.(map[string]any)
	if m["id"].(int64) != 1 || m["name"].(string) != "alice" {
		t.Fatalf("generic=%v", m)
	}
}
