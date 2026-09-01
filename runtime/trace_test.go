package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFrameHdrRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	hdr := make([]byte, 24)
	hdr[0], hdr[23] = 1, 2
	if err := writeFrame(&buf, message{typ: MsgCall, seq: 7, method: "GetUser", hdr: hdr, body: []byte("hi")}); err != nil {
		t.Fatal(err)
	}
	msg, err := readMsg(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if msg.method != "GetUser" || string(msg.body) != "hi" || len(msg.hdr) != 24 || msg.hdr[0] != 1 || msg.hdr[23] != 2 {
		t.Fatalf("%+v", msg)
	}
}

func TestTraceOrderToUser(t *testing.T) {
	var mu sync.Mutex
	var dumped []zipkinSpan
	got := make(chan struct{}, 8)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var spans []zipkinSpan
		if err := json.Unmarshal(b, &spans); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		dumped = append(dumped, spans...)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		got <- struct{}{}
	}))
	defer ts.Close()

	old := zipkinURL
	zipkinURL = ts.URL
	defer func() { zipkinURL = old }()

	user := NewServer()
	user.Handle("GetUser", func(ctx context.Context, body []byte) ([]byte, error) {
		return []byte("alice"), nil
	})
	uln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer uln.Close()
	go user.Serve(uln)

	order := NewServer()
	order.Handle("GetOrder", func(ctx context.Context, _ []byte) ([]byte, error) {
		cli, err := Dial(uln.Addr().String())
		if err != nil {
			return nil, err
		}
		defer cli.Close()
		return cli.Call(ctx, "GetUser", nil)
	})
	oln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer oln.Close()
	go order.Serve(oln)

	cli, err := Dial(oln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	if _, err := cli.Call(context.Background(), "GetOrder", nil); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-got:
		case <-deadline:
			t.Fatal("timeout waiting spans")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(dumped) != 2 {
		t.Fatalf("spans=%d %+v", len(dumped), dumped)
	}
	byName := map[string]zipkinSpan{}
	for _, s := range dumped {
		byName[s.Name] = s
	}
	parent, child := byName["GetOrder"], byName["GetUser"]
	if parent.TraceID == "" || parent.TraceID != child.TraceID {
		t.Fatalf("trace mismatch %+v %+v", parent, child)
	}
	if child.ParentID != parent.ID {
		t.Fatalf("parent=%s child.parent=%s", parent.ID, child.ParentID)
	}
}
