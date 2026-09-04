package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/vanguard"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	v1 "minikitex/gen/platform/v1"
	"minikitex/gen/platform/v1/platformv1connect"
)

func (s *server) serveHTTP(addr string) error {
	path, handler := platformv1connect.NewPlatformServiceHandler(s, connect.WithInterceptors(authInterceptor{}))
	transcoder, err := vanguard.NewTranscoder([]*vanguard.Service{
		vanguard.NewService(path, handler),
	})
	if err != nil {
		return err
	}
	var h http.Handler = transcoder
	if dir := strings.TrimSpace(os.Getenv("WEB_DIR")); dir != "" {
		fs := http.FileServer(http.Dir(dir))
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/platform.v1.") {
				transcoder.ServeHTTP(w, r)
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
	log.Println("platform", addr)
	return http.ListenAndServe(addr, h2c.NewHandler(h, &http2.Server{}))
}

type authInterceptor struct{}

func (authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := withUser(ctx, req.Spec().Procedure, req.Header().Get("Authorization"))
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := withUser(ctx, conn.Spec().Procedure, conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func withUser(ctx context.Context, procedure, authz string) (context.Context, error) {
	if procedure == platformv1connect.PlatformServiceLoginProcedure {
		return ctx, nil
	}
	user, err := parseToken(authz)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, userKey{}, user), nil
}

func (s *server) watchLive(ctx context.Context, id int64, stream *connect.ServerStream[v1.RunEvent], load func(int64) (*v1.Build, error)) error {
	send := func(ev *v1.RunEvent) error { return stream.Send(ev) }
	doneFrom := func(b *v1.Build) *v1.RunEvent {
		ev := &v1.RunEvent{Done: true, Status: b.GetStatus(), Error: b.GetError()}
		if ev.Error == "" && ev.Status != "" && ev.Status != "ok" && ev.Status != "running" {
			ev.Error = ev.Status
		}
		return ev
	}
	if v, ok := lives.Load(id); ok {
		live := v.(*liveRun)
		snap, ch, cancel := live.subscribe()
		defer cancel()
		if snap != "" {
			if err := send(&v1.RunEvent{Text: snap}); err != nil {
				return err
			}
		}
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case delta, ok := <-ch:
				if !ok {
					b, err := load(id)
					if err != nil {
						return err
					}
					return send(doneFrom(b))
				}
				if delta != "" {
					if err := send(&v1.RunEvent{Text: delta}); err != nil {
						return err
					}
				}
			}
		}
	}
	b, err := load(id)
	if err != nil {
		return err
	}
	if b.GetLog() != "" {
		if err := send(&v1.RunEvent{Text: b.GetLog()}); err != nil {
			return err
		}
	}
	if b.GetStatus() != "running" {
		return send(doneFrom(b))
	}
	prev := b.GetLog()
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			b, err = load(id)
			if err != nil {
				return err
			}
			if logText := b.GetLog(); logText != prev {
				if err := send(&v1.RunEvent{Text: strings.TrimPrefix(logText, prev)}); err != nil {
					return err
				}
				prev = logText
			}
			if b.GetStatus() != "running" {
				return send(doneFrom(b))
			}
			if v, ok := lives.Load(id); ok {
				return s.followLive(ctx, v.(*liveRun), prev, stream, func() (*v1.Build, error) { return load(id) })
			}
		}
	}
}

func (s *server) followLive(ctx context.Context, live *liveRun, already string, stream *connect.ServerStream[v1.RunEvent], load func() (*v1.Build, error)) error {
	snap, ch, cancel := live.subscribe()
	defer cancel()
	if rest := strings.TrimPrefix(snap, already); rest != "" {
		if err := stream.Send(&v1.RunEvent{Text: rest}); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delta, ok := <-ch:
			if !ok {
				b, err := load()
				if err != nil {
					return err
				}
				ev := &v1.RunEvent{Done: true, Status: b.GetStatus(), Error: b.GetError()}
				if ev.Error == "" && ev.Status != "" && ev.Status != "ok" {
					ev.Error = ev.Status
				}
				return stream.Send(ev)
			}
			if delta != "" {
				if err := stream.Send(&v1.RunEvent{Text: delta}); err != nil {
					return err
				}
			}
		}
	}
}
