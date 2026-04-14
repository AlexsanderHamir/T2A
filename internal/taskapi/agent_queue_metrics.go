package taskapi

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/prometheus/client_golang/prometheus"
)

var registerAgentQueueMetrics sync.Once

// registerAgentQueueMetricsOn registers depth and capacity gauges on reg (tests may use a dedicated registry).
func registerAgentQueueMetricsOn(reg prometheus.Registerer, q *agents.MemoryQueue) error {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.registerAgentQueueMetricsOn")
	if q == nil {
		return fmt.Errorf("taskapi: nil MemoryQueue")
	}
	depth := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "taskapi",
		Name:      "agent_queue_depth",
		Help:      "Number of ready tasks currently buffered in the in-process agent queue.",
	}, func() float64 {
		return float64(q.BufferDepth())
	})
	capG := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi",
		Name:      "agent_queue_capacity",
		Help:      "Configured maximum buffer size for the in-process agent queue.",
	})
	if err := reg.Register(depth); err != nil {
		return fmt.Errorf("register agent_queue_depth: %w", err)
	}
	if err := reg.Register(capG); err != nil {
		return fmt.Errorf("register agent_queue_capacity: %w", err)
	}
	capG.Set(float64(q.BufferCap()))
	return nil
}

// RegisterAgentQueueMetrics registers taskapi_agent_queue_depth and taskapi_agent_queue_capacity
// on the default Prometheus registry. Safe to call once per process.
func RegisterAgentQueueMetrics(q *agents.MemoryQueue) {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.RegisterAgentQueueMetrics")
	if q == nil {
		return
	}
	registerAgentQueueMetrics.Do(func() {
		if err := registerAgentQueueMetricsOn(prometheus.DefaultRegisterer, q); err != nil {
			var dup prometheus.AlreadyRegisteredError
			if errors.As(err, &dup) {
				return
			}
			slog.Warn("prometheus agent queue metrics register failed", "cmd", cmdLog, "operation", "taskapi.RegisterAgentQueueMetrics", "err", err)
			return
		}
		slog.Info("prometheus agent queue metrics registered", "cmd", cmdLog, "operation", "taskapi.RegisterAgentQueueMetrics",
			"capacity", q.BufferCap())
	})
}
