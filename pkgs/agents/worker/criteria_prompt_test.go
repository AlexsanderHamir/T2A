package worker

import (
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestInjectCriteria_NoAlreadyVerified_RendersAllItems(t *testing.T) {
	items := []store.ChecklistVerifyItem{
		{ID: "c1", Text: "first criterion"},
		{ID: "c2", Text: "second criterion"},
	}
	reportPath := "/tmp/t2a-worker/cycle-1/criteria-report.json"
	out := injectCriteria("base", items, "cycle-1", reportPath, nil)
	if !strings.Contains(out, "first criterion") || !strings.Contains(out, "second criterion") {
		t.Fatalf("expected all items in prompt, got:\n%s", out)
	}
	if strings.Contains(out, "Already verified") {
		t.Errorf("unexpected Already-verified header when no locked passes; out=%q", out)
	}
	// PR1 contract: the prompt MUST render the absolute,
	// worker-managed report path, never the legacy .t2a/<cycleID>/...
	// relative path that lived inside RepoRoot.
	if !strings.Contains(out, reportPath) {
		t.Fatalf("absolute report path missing from prompt; want=%q got=%s", reportPath, out)
	}
	if strings.Contains(out, ".t2a/") {
		t.Fatalf(".t2a/ relative path leaked into prompt:\n%s", out)
	}
}

// TestInjectCriteria_LockedItem_OmittedFromActiveChecklist pins the
// retry-efficiency contract: a criterion already proven passed in an
// earlier attempt MUST appear under the "Already verified" header and
// NOT in the active checklist. Otherwise the agent re-does work and
// the verifier re-evaluates settled items.
func TestInjectCriteria_LockedItem_OmittedFromActiveChecklist(t *testing.T) {
	items := []store.ChecklistVerifyItem{
		{ID: "c1", Text: "first criterion"},
		{ID: "c2", Text: "second criterion"},
	}
	already := map[string]criterionVerdict{
		"c1": {id: "c1", passed: true},
	}
	out := injectCriteria("base", items, "cycle-1", "/tmp/t2a-worker/cycle-1/criteria-report.json", already)

	if !strings.Contains(out, "## Already verified") {
		t.Fatalf("missing Already-verified header in:\n%s", out)
	}
	if !strings.Contains(out, "[c1] first criterion") {
		t.Fatalf("locked item missing from header in:\n%s", out)
	}

	// Active section must come after the locked-passes header. Within
	// the active section, c1 must NOT appear; c2 must.
	idxLocked := strings.Index(out, "## Already verified")
	idxActive := strings.Index(out, "## Done criteria")
	if idxActive == -1 || idxActive < idxLocked {
		t.Fatalf("active checklist must come after locked header; out=%q", out)
	}
	active := out[idxActive:]
	if strings.Contains(active, "[c1]") {
		t.Errorf("locked item leaked into active checklist:\n%s", active)
	}
	if !strings.Contains(active, "[c2]") {
		t.Errorf("active item missing from active checklist:\n%s", active)
	}
}

// TestInjectCriteria_AllLocked_NoActiveSchema pins the corner case
// where every criterion is already verified. The active checklist
// must collapse to a no-op note and must NOT instruct the agent to
// produce a criteria-report (there are no IDs to report on).
func TestInjectCriteria_AllLocked_NoActiveSchema(t *testing.T) {
	items := []store.ChecklistVerifyItem{
		{ID: "c1", Text: "first"},
	}
	already := map[string]criterionVerdict{
		"c1": {id: "c1", passed: true},
	}
	out := injectCriteria("base", items, "cycle-1", "/tmp/t2a-worker/cycle-1/criteria-report.json", already)
	if strings.Contains(out, "Schema:") {
		t.Errorf("schema instructions must be omitted when nothing is active:\n%s", out)
	}
	if !strings.Contains(out, "All criteria are already verified") {
		t.Errorf("expected all-locked sentinel; got:\n%s", out)
	}
}
