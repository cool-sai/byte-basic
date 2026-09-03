package runtime

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsExpose(t *testing.T) {
	rpcRequests.Reset()
	rpcDuration.Reset()
	rpcRequests.WithLabelValues("GetUser", "ok").Inc()
	rpcRequests.WithLabelValues("GetUser", "ok").Inc()
	rpcRequests.WithLabelValues("GetUser", "error").Inc()
	rpcDuration.WithLabelValues("GetUser").Observe(0.002)
	rpcDuration.WithLabelValues("GetUser").Observe(0.00005)
	rpcDuration.WithLabelValues("GetUser").Observe(0.01)

	if err := testutil.CollectAndCompare(rpcRequests, strings.NewReader(`
# HELP rpc_requests_total RPC calls handled
# TYPE rpc_requests_total counter
rpc_requests_total{method="GetUser",status="error"} 1
rpc_requests_total{method="GetUser",status="ok"} 2
`)); err != nil {
		t.Fatal(err)
	}
	if err := testutil.CollectAndCompare(rpcDuration, strings.NewReader(`
# HELP rpc_request_duration_seconds RPC handler latency
# TYPE rpc_request_duration_seconds histogram
rpc_request_duration_seconds_bucket{method="GetUser",le="0.0001"} 1
rpc_request_duration_seconds_bucket{method="GetUser",le="0.0005"} 1
rpc_request_duration_seconds_bucket{method="GetUser",le="0.001"} 1
rpc_request_duration_seconds_bucket{method="GetUser",le="0.005"} 2
rpc_request_duration_seconds_bucket{method="GetUser",le="0.01"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="0.025"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="0.05"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="0.1"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="0.25"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="0.5"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="1"} 3
rpc_request_duration_seconds_bucket{method="GetUser",le="+Inf"} 3
rpc_request_duration_seconds_sum{method="GetUser"} 0.01205
rpc_request_duration_seconds_count{method="GetUser"} 3
`)); err != nil {
		t.Fatal(err)
	}
}
