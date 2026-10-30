package gateway

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
)

// Metrics is the gateway's process-local metric set.
type Metrics struct {
	ConnectionsTotal  atomic.Uint64
	ConnectionsActive atomic.Int64
	AuthFailures      atomic.Uint64
	CommandsTotal     atomic.Uint64
	CommandErrors     atomic.Uint64
	PushesTotal       atomic.Uint64
	BackendErrors     atomic.Uint64
}

// NewMetrics returns an empty metric set.
func NewMetrics() *Metrics { return &Metrics{} }

// Prometheus renders counters/gauges in text exposition format.
func (m *Metrics) Prometheus() string {
	values := map[string]int64{
		"hapegateway_connections_total":    int64(m.ConnectionsTotal.Load()),
		"hapegateway_connections_active":   m.ConnectionsActive.Load(),
		"hapegateway_auth_failures_total":  int64(m.AuthFailures.Load()),
		"hapegateway_commands_total":       int64(m.CommandsTotal.Load()),
		"hapegateway_command_errors_total": int64(m.CommandErrors.Load()),
		"hapegateway_pushes_total":         int64(m.PushesTotal.Load()),
		"hapegateway_backend_errors_total": int64(m.BackendErrors.Load()),
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		typ := "counter"
		if strings.HasSuffix(k, "_active") {
			typ = "gauge"
		}
		fmt.Fprintf(&b, "# TYPE %s %s\n%s %d\n", k, typ, k, values[k])
	}
	return b.String()
}

// Handler serves the Prometheus text endpoint.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(m.Prometheus()))
	}
}
