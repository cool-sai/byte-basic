package runtime

import (
	"strings"
	"testing"
)

func TestMetricsExpose(t *testing.T) {
	m := &metricSet{by: map[string]*rpcStat{}}
	m.observe("GetUser", "ok", 0.002)
	m.observe("GetUser", "ok", 0.00005)
	m.observe("GetUser", "error", 0.01)

	out := string(m.expose())
	for _, want := range []string{
		`rpc_requests_total{method="GetUser",status="ok"} 2`,
		`rpc_requests_total{method="GetUser",status="error"} 1`,
		`rpc_request_duration_seconds_bucket{method="GetUser",le="0.0001"} 1`,
		`rpc_request_duration_seconds_bucket{method="GetUser",le="0.005"} 2`,
		`rpc_request_duration_seconds_bucket{method="GetUser",le="+Inf"} 3`,
		`rpc_request_duration_seconds_count{method="GetUser"} 3`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s\n%s", want, out)
		}
	}
}
