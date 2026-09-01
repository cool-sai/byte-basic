package runtime

import (
	"context"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
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
}

func NewServer() *Server {
	return &Server{handlers: map[string]HandlerFunc{}}
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
		s.mu.RUnlock()
		start := time.Now()
		if h == nil {
			observeRPC(msg.method, "error", time.Since(start))
			_ = writeMsg(conn, MsgException, msg.seq, msg.method, []byte("unknown method "+msg.method))
			continue
		}
		body, err := h(context.Background(), msg.body)
		if err != nil {
			observeRPC(msg.method, "error", time.Since(start))
			slog.Error("rpc", "method", msg.method, "seq", msg.seq, "err", err)
			_ = writeMsg(conn, MsgException, msg.seq, msg.method, []byte(err.Error()))
			continue
		}
		observeRPC(msg.method, "ok", time.Since(start))
		slog.Info("rpc", "method", msg.method, "seq", msg.seq)
		_ = writeMsg(conn, MsgReply, msg.seq, msg.method, body)
	}
}
