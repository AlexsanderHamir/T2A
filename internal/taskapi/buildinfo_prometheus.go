package taskapi

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/AlexsanderHamir/T2A/internal/version"
	"github.com/prometheus/client_golang/prometheus"
)

var registerBuildInfo sync.Once

// RegisterBuildInfoGauge registers taskapi_build_info{version,revision,go_version}=1
// on the default Prometheus registry. Labels come from version.PrometheusBuildInfoLabels.
func RegisterBuildInfoGauge() {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.RegisterBuildInfoGauge")
	registerBuildInfo.Do(func() {
		vec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "taskapi",
				Name:      "build_info",
				Help:      "Build metadata; value is always 1. version matches health JSON version field; revision is short vcs.revision when embedded.",
			},
			[]string{"version", "revision", "go_version"},
		)
		if err := prometheus.DefaultRegisterer.Register(vec); err != nil {
			var dup prometheus.AlreadyRegisteredError
			if errors.As(err, &dup) {
				return
			}
			slog.Warn("prometheus build_info register failed", "cmd", cmdLog, "operation", "taskapi.RegisterBuildInfoGauge", "err", err)
			return
		}
		v, r, gv := version.PrometheusBuildInfoLabels()
		vec.WithLabelValues(v, r, gv).Set(1)
		slog.Info("prometheus build_info registered", "cmd", cmdLog, "operation", "taskapi.RegisterBuildInfoGauge",
			"version", v, "revision", r, "go_version", gv)
	})
}
