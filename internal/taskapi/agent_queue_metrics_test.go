package taskapi

import (
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestRegisterAgentQueueMetricsOn_depthAndCapacity(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	q := agents.NewMemoryQueue(4)
	if err := registerAgentQueueMetricsOn(reg, q); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	t1 := domain.Task{
		ID: "a1", Title: "t", InitialPrompt: "",
		Status: domain.StatusReady, Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral,
	}
	if err := q.NotifyReadyTask(ctx, t1); err != nil {
		t.Fatal(err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var depthVal, capVal float64
	var foundDepth, foundCap bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "taskapi_agent_queue_depth":
			foundDepth = true
			depthVal = gaugeValue(mf)
		case "taskapi_agent_queue_capacity":
			foundCap = true
			capVal = gaugeValue(mf)
		}
	}
	if !foundDepth || !foundCap {
		t.Fatal("expected taskapi_agent_queue_depth and taskapi_agent_queue_capacity")
	}
	if depthVal != 1 {
		t.Fatalf("depth: got %v want 1", depthVal)
	}
	if capVal != 4 {
		t.Fatalf("capacity: got %v want 4", capVal)
	}
}

func gaugeValue(mf *dto.MetricFamily) float64 {
	ms := mf.GetMetric()
	if len(ms) == 0 {
		return 0
	}
	if g := ms[0].GetGauge(); g != nil {
		return g.GetValue()
	}
	return 0
}
