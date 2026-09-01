package runtime

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestLoggerCarriesTrace(t *testing.T) {
	var hdr []byte
	s := NewServer()
	s.Handle("Ping", func(ctx context.Context, _ []byte) ([]byte, error) {
		hdr = outgoingHdr(ctx)
		return nil, nil
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go s.Serve(ln)
	cli, err := Dial(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	if _, err := cli.Call(context.Background(), "Ping", nil); err != nil {
		t.Fatal(err)
	}
	if len(hdr) != 24 {
		t.Fatalf("trace not on ctx, hdr len=%d", len(hdr))
	}
}

func TestMiddlewareOrder(t *testing.T) {
	var steps []string
	mw := func(name string) Middleware {
		return func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, body []byte) ([]byte, error) {
				steps = append(steps, name+"-in")
				resp, err := next(ctx, body)
				steps = append(steps, name+"-out")
				return resp, err
			}
		}
	}
	s := &Server{handlers: map[string]HandlerFunc{}}
	s.Use(mw("a"), mw("b"))
	s.Handle("Ping", func(ctx context.Context, body []byte) ([]byte, error) {
		steps = append(steps, "h")
		return body, nil
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go s.Serve(ln)
	cli, err := Dial(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	if _, err := cli.Call(context.Background(), "Ping", nil); err != nil {
		t.Fatal(err)
	}
	want := "a-in b-in h b-out a-out"
	if got := strings.Join(steps, " "); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
