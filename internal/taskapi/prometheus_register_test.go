package taskapi

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterDefaultPrometheusCollectors_idempotent(t *testing.T) {
	RegisterDefaultPrometheusCollectors()
	RegisterDefaultPrometheusCollectors()

	var names []string
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.Name == nil {
			continue
		}
		names = append(names, mf.GetName())
	}
	body := strings.Join(names, "\n")
	for _, needle := range []string{"go_goroutines", "process_cpu_seconds_total", "go_memstats_alloc_bytes"} {
		if !strings.Contains(body, needle) {
			t.Errorf("expected metric name containing %q in gathered families; sample:\n%s", needle, body[:min(len(body), 800)])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
