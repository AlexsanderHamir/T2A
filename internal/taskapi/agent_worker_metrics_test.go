package taskapi

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestRegisterAgentWorkerMetricsOn_counterAndHistogram(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	adapter, err := registerAgentWorkerMetricsOn(reg)
	if err != nil {
		t.Fatal(err)
	}

	adapter.RecordRun("cursor", "succeeded", 750*time.Millisecond)
	adapter.RecordRun("cursor", "failed", 5*time.Second)
	adapter.RecordRun("cursor", "succeeded", 12*time.Second)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	var foundCounter, foundHistogram bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "t2a_agent_runs_total":
			foundCounter = true
			assertCounterValue(t, mf, map[string]string{"runner": "cursor", "terminal_status": "succeeded"}, 2)
			assertCounterValue(t, mf, map[string]string{"runner": "cursor", "terminal_status": "failed"}, 1)
		case "t2a_agent_run_duration_seconds":
			foundHistogram = true
			assertHistogramSampleCount(t, mf, map[string]string{"runner": "cursor"}, 3)
		}
	}
	if !foundCounter {
		t.Fatal("expected t2a_agent_runs_total in registry")
	}
	if !foundHistogram {
		t.Fatal("expected t2a_agent_run_duration_seconds in registry")
	}
}

func TestRegisterAgentWorkerMetricsOn_bucketsCoverV1RunTimeoutRange(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	adapter, err := registerAgentWorkerMetricsOn(reg)
	if err != nil {
		t.Fatal(err)
	}
	// One observation forces the histogram to emit its bucket layout
	// in the Gather output (Prometheus prunes empty histograms).
	adapter.RecordRun("cursor", "succeeded", time.Second)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var maxBucket float64
	for _, mf := range mfs {
		if mf.GetName() != "t2a_agent_run_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			h := m.GetHistogram()
			if h == nil {
				continue
			}
			for _, b := range h.GetBucket() {
				if b.GetUpperBound() > maxBucket {
					maxBucket = b.GetUpperBound()
				}
			}
		}
	}
	if maxBucket < 1800 {
		t.Fatalf("max bucket = %v, want >= 1800 to cover 30m run timeout", maxBucket)
	}
}

func assertCounterValue(t *testing.T, mf *dto.MetricFamily, labels map[string]string, want float64) {
	t.Helper()
	for _, m := range mf.GetMetric() {
		if matchLabels(m.GetLabel(), labels) {
			if got := m.GetCounter().GetValue(); got != want {
				t.Fatalf("counter %s%v = %v, want %v", mf.GetName(), labels, got, want)
			}
			return
		}
	}
	t.Fatalf("counter %s%v: no matching metric", mf.GetName(), labels)
}

func assertHistogramSampleCount(t *testing.T, mf *dto.MetricFamily, labels map[string]string, want uint64) {
	t.Helper()
	for _, m := range mf.GetMetric() {
		if matchLabels(m.GetLabel(), labels) {
			if got := m.GetHistogram().GetSampleCount(); got != want {
				t.Fatalf("histogram %s%v sample_count = %v, want %v", mf.GetName(), labels, got, want)
			}
			return
		}
	}
	t.Fatalf("histogram %s%v: no matching metric", mf.GetName(), labels)
}

func matchLabels(got []*dto.LabelPair, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, lp := range got {
		if v, ok := want[lp.GetName()]; !ok || v != lp.GetValue() {
			return false
		}
	}
	return true
}
