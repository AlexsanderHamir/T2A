package taskapi

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterBuildInfoGauge_exposesFamily(t *testing.T) {
	RegisterBuildInfoGauge()

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, mf := range mfs {
		if mf.GetName() != "taskapi_build_info" {
			continue
		}
		found = true
		ms := mf.GetMetric()
		if len(ms) != 1 {
			t.Fatalf("taskapi_build_info: want 1 series, got %d", len(ms))
		}
		labels := map[string]string{}
		for _, lp := range ms[0].GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		for _, key := range []string{"version", "revision", "go_version"} {
			if labels[key] == "" {
				t.Fatalf("missing label %q: %#v", key, labels)
			}
		}
		if !strings.HasPrefix(labels["go_version"], "go") {
			t.Fatalf("go_version: %q", labels["go_version"])
		}
		if ms[0].GetGauge().GetValue() != 1 {
			t.Fatalf("gauge value: %v", ms[0].GetGauge().GetValue())
		}
	}
	if !found {
		t.Fatal("expected taskapi_build_info in default gatherer")
	}
}
