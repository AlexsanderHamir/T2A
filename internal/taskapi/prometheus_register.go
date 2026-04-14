package taskapi

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var defaultPromCollectors sync.Once

// RegisterDefaultPrometheusCollectors registers standard Go runtime and process
// collectors on prometheus.DefaultRegisterer (the registry used by GET /metrics).
// It is safe to call multiple times; registration runs once per process.
func RegisterDefaultPrometheusCollectors() {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.RegisterDefaultPrometheusCollectors")
	defaultPromCollectors.Do(func() {
		reg := prometheus.DefaultRegisterer
		for _, c := range []prometheus.Collector{
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		} {
			if err := reg.Register(c); err != nil {
				var dup prometheus.AlreadyRegisteredError
				if errors.As(err, &dup) {
					continue
				}
				slog.Warn("prometheus collector register failed", "cmd", cmdLog, "operation", "taskapi.prometheus_register", "err", err)
				continue
			}
		}
		slog.Info("prometheus default collectors registered", "cmd", cmdLog, "operation", "taskapi.prometheus_register",
			"go_collector", true, "process_collector", true)
	})
}
