package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestHTTP_getChecklist_envelopeShape pins the documented GET 200 envelope:
// only the `items` key at top level, each item exactly `{id, sort_order, text, done}`.
// `done` is a JSON boolean (not a string or number) and `sort_order` is a JSON number.
func TestHTTP_getChecklist_envelopeShape(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	taskID := mustCreateChecklistTask(t, srv, "chk-get-shape")
	if _, err := st.AddChecklistItem(context.Background(), taskID, "alpha", domain.ActorUser); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(srv.URL + "/tasks/" + taskID + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if _, ok := top["items"]; !ok || len(top) != 1 {
		t.Fatalf("GET checklist 200 must return only `items` (docs/API-HTTP.md); got keys=%v body=%s", keysOf(top), raw)
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(top["items"], &items); err != nil {
		t.Fatalf("items not a JSON array: %v body=%s", err, raw)
	}
	if len(items) != 1 {
		t.Fatalf("items len=%d want 1", len(items))
	}
	wantKeys := map[string]struct{}{"id": {}, "sort_order": {}, "text": {}, "done": {}}
	for k := range wantKeys {
		if _, ok := items[0][k]; !ok {
			t.Errorf("item missing key %q (docs/API-HTTP.md): %s", k, items[0])
		}
	}
	for k := range items[0] {
		if _, ok := wantKeys[k]; !ok {
			t.Errorf("item has unexpected key %q (docs/API-HTTP.md): %s", k, items[0])
		}
	}

	var typed struct {
		Items []struct {
			ID        string `json:"id"`
			SortOrder int    `json:"sort_order"`
			Text      string `json:"text"`
			Done      bool   `json:"done"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("decode typed: %v body=%s", err, raw)
	}
	if got := typed.Items[0]; got.ID == "" || got.Text != "alpha" || got.SortOrder < 1 || got.Done {
		t.Fatalf("item=%+v want non-empty id, text=alpha, sort_order>=1, done=false", got)
	}
}

// TestHTTP_getChecklist_emptyItemsIsArrayNotNull pins the documented invariant:
// `items` is always a JSON array (`[]` when none, never `null` or omitted).
// We assert directly on the raw JSON so a future change to `var items []…` (which
// marshals to `null`) would fail loudly.
func TestHTTP_getChecklist_emptyItemsIsArrayNotNull(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateChecklistTask(t, srv, "chk-empty-get")

	res, err := http.Get(srv.URL + "/tasks/" + taskID + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	itemsRaw, ok := top["items"]
	if !ok {
		t.Fatalf("`items` key missing (must be present even when empty): %s", raw)
	}
	if string(itemsRaw) != "[]" {
		t.Fatalf("items raw=%s want `[]` exactly (not null/omitted)", itemsRaw)
	}
}

// TestHTTP_getChecklist_orderIsSortOrderAscThenIDAsc pins the documented stable
// ordering `sort_order ASC, id ASC`. We add three items and assert their
// returned order matches their insertion order (since AddChecklistItem assigns
// `MAX(sort_order)+1`, insertion order == sort_order order).
func TestHTTP_getChecklist_orderIsSortOrderAscThenIDAsc(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	taskID := mustCreateChecklistTask(t, srv, "chk-order")
	ctx := context.Background()
	wantTexts := []string{"first", "second", "third"}
	wantIDs := make([]string, 0, 3)
	for _, txt := range wantTexts {
		it, err := st.AddChecklistItem(ctx, taskID, txt, domain.ActorUser)
		if err != nil {
			t.Fatal(err)
		}
		wantIDs = append(wantIDs, it.ID)
	}

	res, err := http.Get(srv.URL + "/tasks/" + taskID + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var got struct {
		Items []struct {
			ID        string `json:"id"`
			SortOrder int    `json:"sort_order"`
			Text      string `json:"text"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != len(wantTexts) {
		t.Fatalf("items len=%d want %d", len(got.Items), len(wantTexts))
	}
	if !sort.SliceIsSorted(got.Items, func(i, j int) bool {
		if got.Items[i].SortOrder != got.Items[j].SortOrder {
			return got.Items[i].SortOrder < got.Items[j].SortOrder
		}
		return got.Items[i].ID < got.Items[j].ID
	}) {
		t.Fatalf("items not sorted by sort_order ASC, id ASC: %+v", got.Items)
	}
	for i, it := range got.Items {
		if it.Text != wantTexts[i] {
			t.Errorf("items[%d].text=%q want %q (sort order should follow insertion order)", i, it.Text, wantTexts[i])
		}
	}
}

// TestHTTP_getChecklist_404OnUnknownTask pins the documented 404 mapping.
// This route is read-only and, unlike the write surface, did not previously
// have a dedicated HTTP-level test pinning the missing-task response code.
func TestHTTP_getChecklist_404OnUnknownTask(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	const ghost = "11111111-1111-4111-8111-111111111111"
	res, err := http.Get(srv.URL + "/tasks/" + ghost + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, body)
	}
}

// TestHTTP_getChecklist_inheritanceAndPerSubjectDone pins the most subtle
// documented behavior: an inheriting child returns its parent's definitions
// (resolved through `checklist_inherit`), but `done` is filtered to the
// **child's** completion rows. So when the child agent-marks one definition
// done, the child's GET shows it done while the parent's GET shows the same
// definition still undone — independent per-subject done state.
func TestHTTP_getChecklist_inheritanceAndPerSubjectDone(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()
	parentID := mustCreateChecklistTask(t, srv, "chk-inh-parent")
	a, err := st.AddChecklistItem(ctx, parentID, "shared-A", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.AddChecklistItem(ctx, parentID, "shared-B", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	childID := mustCreateChildInheriting(t, srv, parentID, "chk-inh-child")

	if err := st.SetChecklistItemDone(ctx, childID, a.ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	checkChildGET := func(t *testing.T) {
		t.Helper()
		got := getChecklistAsMap(t, srv.URL, childID)
		if !got[a.ID] {
			t.Errorf("child GET: %q must be done (set on child)", a.ID)
		}
		if got[b.ID] {
			t.Errorf("child GET: %q must NOT be done", b.ID)
		}
	}
	checkParentGET := func(t *testing.T) {
		t.Helper()
		got := getChecklistAsMap(t, srv.URL, parentID)
		if got[a.ID] {
			t.Errorf("parent GET: %q must NOT be done — completion was recorded against child only (per-subject done)", a.ID)
		}
		if got[b.ID] {
			t.Errorf("parent GET: %q must NOT be done", b.ID)
		}
	}

	t.Run("childInheritsAndOwnsDoneState", checkChildGET)
	t.Run("parentRowsRemainUndone", checkParentGET)
}

func getChecklistAsMap(t *testing.T, baseURL, taskID string) map[string]bool {
	t.Helper()
	res, err := http.Get(baseURL + "/tasks/" + taskID + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET checklist %s status %d body=%s", taskID, res.StatusCode, body)
	}
	var got struct {
		Items []struct {
			ID   string `json:"id"`
			Done bool   `json:"done"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]bool, len(got.Items))
	for _, it := range got.Items {
		out[it.ID] = it.Done
	}
	return out
}
