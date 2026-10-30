package gateway

import (
	"strings"
	"testing"
)

func TestMetricsPrometheus(t *testing.T) {
	m := NewMetrics()
	m.ConnectionsTotal.Add(2)
	m.ConnectionsActive.Store(1)
	m.CommandsTotal.Add(5)
	m.CommandErrors.Add(1)
	out := m.Prometheus()
	for _, want := range []string{
		"# TYPE hapegateway_connections_total counter",
		"hapegateway_connections_total 2",
		"# TYPE hapegateway_connections_active gauge",
		"hapegateway_connections_active 1",
		"hapegateway_commands_total 5",
		"hapegateway_command_errors_total 1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
