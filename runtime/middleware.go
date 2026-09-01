package runtime

import (
	"context"
	"log/slog"
	"time"
)

type Middleware func(HandlerFunc) HandlerFunc

type rpcInfo struct {
	method string
	seq    uint32
	hdr    []byte
}

type rpcKey struct{}
type loggerKey struct{}

func withRPC(ctx context.Context, info rpcInfo) context.Context {
	return context.WithValue(ctx, rpcKey{}, info)
}

func infoFrom(ctx context.Context) rpcInfo {
	info, _ := ctx.Value(rpcKey{}).(rpcInfo)
	return info
}

func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

func chain(mws []Middleware, h HandlerFunc) HandlerFunc {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func Trace(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		info := infoFrom(ctx)
		ctx, sp := startSpan(ctx, info.method, info.hdr)
		ctx = context.WithValue(ctx, loggerKey{}, slog.Default().With("trace", sp.hexTrace()))
		resp, err := next(ctx, body)
		sp.finish(err)
		return resp, err
	}
}

func Metrics(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		start := time.Now()
		resp, err := next(ctx, body)
		status := "ok"
		if err != nil {
			status = "error"
		}
		observeRPC(infoFrom(ctx).method, status, time.Since(start))
		return resp, err
	}
}

func Log(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		info := infoFrom(ctx)
		resp, err := next(ctx, body)
		if err != nil {
			Logger(ctx).Error("rpc", "method", info.method, "seq", info.seq, "err", err)
		} else {
			Logger(ctx).Info("rpc", "method", info.method, "seq", info.seq)
		}
		return resp, err
	}
}
