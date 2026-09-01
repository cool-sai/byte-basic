package runtime

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Prometheus histogram buckets, seconds. Local RPC is usually in the first few.
var latencyBuckets = []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}

type rpcStat struct {
	ok, err uint64
	sum     float64
	count   uint64
	buckets []uint64 // cumulative; last is +Inf
}

type metricSet struct {
	mu sync.Mutex
	by map[string]*rpcStat
}

var rpcMetrics = &metricSet{by: map[string]*rpcStat{}}

func observeRPC(method, status string, d time.Duration) {
	rpcMetrics.observe(method, status, d.Seconds())
}

func (m *metricSet) observe(method, status string, sec float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.by[method]
	if s == nil {
		s = &rpcStat{buckets: make([]uint64, len(latencyBuckets)+1)}
		m.by[method] = s
	}
	if status == "ok" {
		s.ok++
	} else {
		s.err++
	}
	s.sum += sec
	s.count++
	i := 0
	for i < len(latencyBuckets) && sec > latencyBuckets[i] {
		i++
	}
	for ; i < len(s.buckets); i++ {
		s.buckets[i]++
	}
}

func (m *metricSet) expose() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	methods := make([]string, 0, len(m.by))
	for k := range m.by {
		methods = append(methods, k)
	}
	sort.Strings(methods)

	var b []byte
	b = append(b, "# HELP rpc_requests_total RPC calls handled\n"...)
	b = append(b, "# TYPE rpc_requests_total counter\n"...)
	for _, method := range methods {
		s := m.by[method]
		b = fmt.Appendf(b, "rpc_requests_total{method=%q,status=\"ok\"} %d\n", method, s.ok)
		b = fmt.Appendf(b, "rpc_requests_total{method=%q,status=\"error\"} %d\n", method, s.err)
	}
	b = append(b, "# HELP rpc_request_duration_seconds RPC handler latency\n"...)
	b = append(b, "# TYPE rpc_request_duration_seconds histogram\n"...)
	for _, method := range methods {
		s := m.by[method]
		for i, le := range latencyBuckets {
			b = fmt.Appendf(b, "rpc_request_duration_seconds_bucket{method=%q,le=%q} %d\n", method, strconv.FormatFloat(le, 'f', -1, 64), s.buckets[i])
		}
		b = fmt.Appendf(b, "rpc_request_duration_seconds_bucket{method=%q,le=\"+Inf\"} %d\n", method, s.buckets[len(latencyBuckets)])
		b = fmt.Appendf(b, "rpc_request_duration_seconds_sum{method=%q} %s\n", method, strconv.FormatFloat(s.sum, 'f', -1, 64))
		b = fmt.Appendf(b, "rpc_request_duration_seconds_count{method=%q} %d\n", method, s.count)
	}
	return b
}

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write(rpcMetrics.expose())
	})
	slog.Info("metrics", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("metrics", "err", err)
	}
}
