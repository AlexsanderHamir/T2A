package handler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	taskapiDomainTasksCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "domain_tasks_created_total",
		Help:      "Tasks successfully persisted via POST /tasks (HTTP 201).",
	})
	taskapiDomainTasksUpdatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "domain_tasks_updated_total",
		Help:      "Tasks successfully updated via PATCH /tasks/{id} (HTTP 200).",
	})
)
