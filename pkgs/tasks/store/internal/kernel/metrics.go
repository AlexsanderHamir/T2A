package kernel

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Fixed op label values for taskapi_store_operation_duration_seconds (low cardinality).
// Each constant names exactly one public Store entrypoint; new methods must add a label here
// rather than reusing one. See pkgs/tasks/store/README.md for the source-of-truth concern map.
const (
	OpCreateTask              = "create_task"
	OpGetTask                 = "get_task"
	OpUpdateTask              = "update_task"
	OpDeleteTask              = "delete_task"
	OpListFlat                = "list_flat"
	OpListRootForest          = "list_root_forest"
	OpListRootForestAfter     = "list_root_forest_after"
	OpGetTaskTree             = "get_task_tree"
	OpListTaskEvents          = "list_task_events"
	OpListTaskEventsPage      = "list_task_events_page"
	OpGetTaskEvent            = "get_task_event"
	OpTaskEventCount          = "task_event_count"
	OpLastEventSeq            = "last_event_seq"
	OpApprovalPending         = "approval_pending"
	OpAppendTaskEvent         = "append_task_event"
	OpAppendTaskEventResponse = "append_task_event_response"
	OpDefinitionSourceTask    = "definition_source_task"
	OpListChecklist           = "list_checklist"
	OpAddChecklistItem        = "add_checklist_item"
	OpDeleteChecklistItem     = "delete_checklist_item"
	OpUpdateChecklistItemText = "update_checklist_item_text"
	OpSetChecklistItemDone    = "set_checklist_item_done"
	OpSaveDraft               = "save_draft"
	OpListDrafts              = "list_drafts"
	OpGetDraft                = "get_draft"
	OpDeleteDraft             = "delete_draft"
	OpEvaluateDraft           = "evaluate_draft"
	OpListDraftEvaluations    = "list_draft_evaluations"
	OpTaskStats               = "task_stats"
	OpPing                    = "ping"
	OpReady                   = "ready"
	OpListReadyQueue          = "list_ready_queue"
	OpListReadyUserCreated    = "list_ready_user_created"
	OpApplyDevTaskRowMirror   = "apply_dev_task_row_mirror"
	OpListDevsimTasks         = "list_devsim_tasks"
	OpStartCycle              = "start_cycle"
	OpTerminateCycle          = "terminate_cycle"
	OpGetCycle                = "get_cycle"
	OpListCyclesForTask       = "list_cycles_for_task"
	OpStartCyclePhase         = "start_cycle_phase"
	OpCompleteCyclePhase      = "complete_cycle_phase"
	OpListCyclePhases         = "list_cycle_phases"
)

// opDurationBuckets favor sub-100ms resolution for SQL point reads and short tx.
var opDurationBuckets = []float64{
	0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// operationDurationSeconds is the single Prometheus histogram for all store entrypoints.
// Owned exclusively by this package so the registration cannot accidentally be duplicated
// when the public store facade splits into multiple subpackages under internal/.
var operationDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "store_operation_duration_seconds",
		Help:      "Duration of store persistence operations in seconds. Label op is a fixed operation name, not raw SQL.",
		Buckets:   opDurationBuckets,
	},
	[]string{"op"},
)

// DeferLatency records wall time for one store API entrypoint on return.
// Intentionally no slog (hot path; see cmd/funclogmeasure skip list).
func DeferLatency(op string) func() {
	start := time.Now()
	return func() {
		operationDurationSeconds.WithLabelValues(op).Observe(time.Since(start).Seconds())
	}
}
