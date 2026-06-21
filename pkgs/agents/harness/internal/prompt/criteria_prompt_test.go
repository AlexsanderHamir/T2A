package prompt

import (
	"strings"
	"testing"
)

func TestInjectCriteria_NoAlreadyVerified_RendersAllItems(t *testing.T) {
	items := []ChecklistItem{
		{ID: "c1", Text: "first criterion"},
		{ID: "c2", Text: "second criterion"},
	}
	reportPath := "/tmp/hamix-worker/cycle-1/criteria-report.json"
	out := InjectCriteria("base", items, reportPath, nil)
	if !strings.Contains(out, "first criterion") || !strings.Contains(out, "second criterion") {
		t.Fatalf("expected all items in prompt, got:\n%s", out)
	}
	if strings.Contains(out, "Already verified") {
		t.Errorf("unexpected Already-verified header when no locked passes; out=%q", out)
	}
	if !strings.Contains(out, reportPath) {
		t.Fatalf("absolute report path missing from prompt; want=%q got=%s", reportPath, out)
	}
	if !strings.Contains(out, "schema_version") {
		t.Fatalf("schema_version missing from prompt:\n%s", out)
	}
	if strings.Contains(out, ".legacy-scratch/") {
		t.Fatalf(".legacy-scratch/ relative path leaked into prompt:\n%s", out)
	}
}

func TestInjectCriteria_LockedItem_OmittedFromActiveChecklist(t *testing.T) {
	items := []ChecklistItem{
		{ID: "c1", Text: "first criterion"},
		{ID: "c2", Text: "second criterion"},
	}
	already := map[string]struct{}{"c1": {}}
	out := InjectCriteria("base", items, "/tmp/hamix-worker/cycle-1/criteria-report.json", already)

	if !strings.Contains(out, "## Already verified") {
		t.Fatalf("missing Already-verified header in:\n%s", out)
	}
	if !strings.Contains(out, "[c1] first criterion") {
		t.Fatalf("locked item missing from header in:\n%s", out)
	}

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

func TestInjectCriteria_AllLocked_NoActiveSchema(t *testing.T) {
	items := []ChecklistItem{
		{ID: "c1", Text: "first"},
	}
	already := map[string]struct{}{"c1": {}}
	out := InjectCriteria("base", items, "/tmp/hamix-worker/cycle-1/criteria-report.json", already)
	if strings.Contains(out, "Schema:") {
		t.Errorf("schema instructions must be omitted when nothing is active:\n%s", out)
	}
	if !strings.Contains(out, "All criteria are already verified") {
		t.Errorf("expected all-locked sentinel; got:\n%s", out)
	}
}
