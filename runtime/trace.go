package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type spanContext struct {
	traceID [16]byte
	spanID  [8]byte
}

type ctxKey struct{}

type span struct {
	name     string
	sc       spanContext
	parent   [8]byte
	hasPar   bool
	start    time.Time
	service  string
	instance string
}

var (
	zipkinURL    = os.Getenv("JAEGER")
	traceSvc     = envOr("PSM", envOr("INSTANCE", "rpc"))
	traceInst    = os.Getenv("INSTANCE")
	zipkinClient = &http.Client{Timeout: 2 * time.Second}
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func startSpan(ctx context.Context, name string, hdr []byte) (context.Context, *span) {
	sp := &span{
		name:     name,
		start:    time.Now(),
		service:  traceSvc,
		instance: traceInst,
	}
	if len(hdr) >= 24 {
		copy(sp.sc.traceID[:], hdr[:16])
		copy(sp.parent[:], hdr[16:24])
		sp.hasPar = sp.parent != [8]byte{}
	} else {
		copy(sp.sc.traceID[:], randN(16))
	}
	copy(sp.sc.spanID[:], randN(8))
	ctx = context.WithValue(ctx, ctxKey{}, sp.sc)
	return ctx, sp
}

func outgoingHdr(ctx context.Context) []byte {
	sc, ok := ctx.Value(ctxKey{}).(spanContext)
	if !ok {
		return nil
	}
	hdr := make([]byte, 24)
	copy(hdr[:16], sc.traceID[:])
	copy(hdr[16:], sc.spanID[:])
	return hdr
}

func (sp *span) hexTrace() string { return hex.EncodeToString(sp.sc.traceID[:]) }

func (sp *span) finish(err error) {
	if zipkinURL == "" {
		return
	}
	dur := time.Since(sp.start).Microseconds()
	if dur < 1 {
		dur = 1
	}
	zs := zipkinSpan{
		ID:        hex.EncodeToString(sp.sc.spanID[:]),
		TraceID:   hex.EncodeToString(sp.sc.traceID[:]),
		Name:      sp.name,
		Timestamp: sp.start.UnixMicro(),
		Duration:  dur,
		Kind:      "SERVER",
		LocalEndpoint: zipkinEP{
			ServiceName: sp.service,
		},
	}
	if sp.hasPar {
		zs.ParentID = hex.EncodeToString(sp.parent[:])
	}
	tags := map[string]string{}
	if sp.instance != "" {
		tags["instance"] = sp.instance
	}
	if err != nil {
		tags["error"] = err.Error()
	}
	if len(tags) > 0 {
		zs.Tags = tags
	}
	go postZipkin(zs)
}

func randN(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

type zipkinEP struct {
	ServiceName string `json:"serviceName"`
}

type zipkinSpan struct {
	ID            string            `json:"id"`
	TraceID       string            `json:"traceId"`
	ParentID      string            `json:"parentId,omitempty"`
	Name          string            `json:"name"`
	Timestamp     int64             `json:"timestamp"`
	Duration      int64             `json:"duration"`
	Kind          string            `json:"kind"`
	LocalEndpoint zipkinEP          `json:"localEndpoint"`
	Tags          map[string]string `json:"tags,omitempty"`
}

func postZipkin(sp zipkinSpan) {
	body, err := json.Marshal([]zipkinSpan{sp})
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, zipkinURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := zipkinClient.Do(req)
	if err != nil {
		slog.Warn("trace export", "err", err)
		return
	}
	resp.Body.Close()
}
