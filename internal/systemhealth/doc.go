// Package systemhealth aggregates a stable JSON snapshot of the
// taskapi process state for the operator-facing GET /system/health
// endpoint. It is *not* the kubelet liveness/readiness probe (those
// stay in pkgs/tasks/handler/handler_health.go); think of it as the
// "what's going on inside the box" view rendered on the
// /observability page.
//
// The aggregator is a pure read over prometheus.DefaultGatherer, so
// every number it returns is the same number that GET /metrics would
// expose — no parallel counters to drift. Counters/gauges that are
// not yet observed surface as zero rather than missing keys, giving
// the SPA a stable shape to render even on a freshly-booted process.
package systemhealth
