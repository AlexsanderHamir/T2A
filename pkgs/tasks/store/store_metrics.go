package store

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Fixed op label values for taskapi_store_operation_duration_seconds (low cardinality).
const (
	storeOpCreateTask              = "create_task"
	storeOpGetTask                 = "get_task"
	storeOpUpdateTask              = "update_task"
	storeOpDeleteTask              = "delete_task"
	storeOpListFlat                = "list_flat"
	storeOpListRootForest          = "list_root_forest"
	storeOpListRootForestAfter     = "list_root_forest_after"
	storeOpGetTaskTree             = "get_task_tree"
	storeOpListTaskEvents          = "list_task_events"
	storeOpListTaskEventsPage      = "list_task_events_page"
	storeOpGetTaskEvent            = "get_task_event"
	storeOpTaskEventCount          = "task_event_count"
	storeOpLastEventSeq            = "last_event_seq"
	storeOpApprovalPending         = "approval_pending"
	storeOpAppendTaskEvent         = "append_task_event"
	storeOpAppendTaskEventResponse = "append_task_event_response"
	storeOpDefinitionSourceTask    = "definition_source_task"
	storeOpListChecklist           = "list_checklist"
	storeOpAddChecklistItem        = "add_checklist_item"
	storeOpDeleteChecklistItem     = "delete_checklist_item"
	storeOpUpdateChecklistItemText = "update_checklist_item_text"
	storeOpSetChecklistItemDone    = "set_checklist_item_done"
	storeOpSaveDraft               = "save_draft"
	storeOpListDrafts              = "list_drafts"
	storeOpGetDraft                = "get_draft"
	storeOpDeleteDraft             = "delete_draft"
	storeOpEvaluateDraft           = "evaluate_draft"
	storeOpListDraftEvaluations    = "list_draft_evaluations"
	storeOpTaskStats               = "task_stats"
	storeOpPing                    = "ping"
	storeOpReady                   = "ready"
	storeOpListReadyQueue          = "list_ready_queue"
	storeOpListReadyUserCreated    = "list_ready_user_created"
	storeOpApplyDevTaskRowMirror   = "apply_dev_task_row_mirror"
	storeOpListDevsimTasks         = "list_devsim_tasks"
	storeOpStartCycle              = "start_cycle"
	storeOpTerminateCycle          = "terminate_cycle"
	storeOpGetCycle                = "get_cycle"
	storeOpListCyclesForTask       = "list_cycles_for_task"
	storeOpStartCyclePhase         = "start_cycle_phase"
	storeOpCompleteCyclePhase      = "complete_cycle_phase"
	storeOpListCyclePhases         = "list_cycle_phases"
)

// storeOpDurationBuckets favor sub-100ms resolution for SQL point reads and short tx.
var storeOpDurationBuckets = []float64{
	0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

var taskapiStoreOperationDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "taskapi",
		Name:      "store_operation_duration_seconds",
		Help:      "Duration of store persistence operations in seconds. Label op is a fixed operation name, not raw SQL.",
		Buckets:   storeOpDurationBuckets,
	},
	[]string{"op"},
)

// deferStoreLatency records wall time for one store API entrypoint on return.
// Intentionally no slog (hot path; see cmd/funclogmeasure skip list).
func deferStoreLatency(op string) func() {
	start := time.Now()
	return func() {
		taskapiStoreOperationDurationSeconds.WithLabelValues(op).Observe(time.Since(start).Seconds())
	}
}
