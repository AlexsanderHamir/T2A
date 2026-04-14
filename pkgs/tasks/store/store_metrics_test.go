package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/prometheus/client_golang/prometheus"
)

func storeOpHistogramSampleCount(op string) (uint64, error) {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0, err
	}
	for _, mf := range mfs {
		if mf.GetName() != "taskapi_store_operation_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			match := false
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "op" && lp.GetValue() == op {
					match = true
					break
				}
			}
			if match {
				h := m.GetHistogram()
				if h == nil {
					continue
				}
				return h.GetSampleCount(), nil
			}
		}
	}
	return 0, nil
}

func TestStore_operation_duration_histogram_create_task(t *testing.T) {
	before, err := storeOpHistogramSampleCount(storeOpCreateTask)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err = s.Create(context.Background(), CreateTaskInput{Title: "hist", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	after, err := storeOpHistogramSampleCount(storeOpCreateTask)
	if err != nil {
		t.Fatal(err)
	}
	if after < before+1 {
		t.Fatalf("create_task histogram sample_count: before=%d after=%d", before, after)
	}
}
