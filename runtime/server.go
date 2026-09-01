package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
)

func init() {
	inst := os.Getenv("INSTANCE")
	if inst == "" {
		inst, _ = os.Hostname()
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("instance", inst)
	if psm := os.Getenv("PSM"); psm != "" {
		logger = logger.With("psm", psm)
	}
	slog.SetDefault(logger)
}

type HandlerFunc func(ctx context.Context, body []byte) ([]byte, error)

type Server struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
	mws      []Middleware
}

func NewServer() *Server {
	s := &Server{handlers: map[string]HandlerFunc{}}
	s.Use(Trace, Metrics, Log)
	return s
}

func (s *Server) Use(mw ...Middleware) {
	s.mu.Lock()
	s.mws = append(s.mws, mw...)
	s.mu.Unlock()
}

func (s *Server) Handle(method string, h HandlerFunc) {
	s.mu.Lock()
	s.handlers[method] = h
	s.mu.Unlock()
}

func (s *Server) ListenAndServe(addr string) error {
	if m := os.Getenv("METRICS"); m != "" {
		go serveMetrics(m)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()
	for {
		msg, err := readMsg(conn)
		if err != nil {
			return
		}
		if msg.typ != MsgCall {
			_ = writeMsg(conn, MsgException, msg.seq, msg.method, []byte("not a call"))
			continue
		}
		s.mu.RLock()
		h := s.handlers[msg.method]
		mws := s.mws
		s.mu.RUnlock()
		if h == nil {
			method := msg.method
			h = func(context.Context, []byte) ([]byte, error) {
				return nil, fmt.Errorf("unknown method %s", method)
			}
		}
		ctx := withRPC(context.Background(), rpcInfo{method: msg.method, seq: msg.seq, hdr: msg.hdr})
		body, err := chain(mws, h)(ctx, msg.body)
		if err != nil {
			_ = writeMsg(conn, MsgException, msg.seq, msg.method, []byte(err.Error()))
			continue
		}
		_ = writeMsg(conn, MsgReply, msg.seq, msg.method, body)
	}
}
