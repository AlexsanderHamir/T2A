package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestHTTP_evaluateDraft_envelopeShape pins the documented response envelope
// from docs/API-HTTP.md: status 201, full top-level key set with no extras,
// scores in 0..100, and the four `sections[]` entries (`title`,
// `initial_prompt`, `priority`, `structure`) in that order, each shaped
// `{key, label, score, summary, suggestions}`.
func TestHTTP_evaluateDraft_envelopeShape(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{
		"id":"evaluate-contract",
		"title":"Improve mention parser reliability",
		"initial_prompt":"Handle nested mentions and malformed ranges with clear errors.",
		"priority":"high",
		"status":"ready",
		"task_type":"feature",
		"checklist_inherit":false,
		"checklist_items":[{"text":"Add parser tests"},{"text":"Document edge cases"}]
	}`
	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201) body=%s", res.StatusCode, raw)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	wantTopKeys := map[string]struct{}{
		"evaluation_id":        {},
		"created_at":           {},
		"overall_score":        {},
		"overall_summary":      {},
		"sections":             {},
		"cohesion_score":       {},
		"cohesion_summary":     {},
		"cohesion_suggestions": {},
	}
	for k := range wantTopKeys {
		if _, ok := top[k]; !ok {
			t.Errorf("missing top-level key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
	for k := range top {
		if _, ok := wantTopKeys[k]; !ok {
			t.Errorf("unexpected top-level key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}

	var env struct {
		EvaluationID        string    `json:"evaluation_id"`
		CreatedAt           time.Time `json:"created_at"`
		OverallScore        int       `json:"overall_score"`
		OverallSummary      string    `json:"overall_summary"`
		CohesionScore       int       `json:"cohesion_score"`
		CohesionSummary     string    `json:"cohesion_summary"`
		CohesionSuggestions []string  `json:"cohesion_suggestions"`
		Sections            []struct {
			Key         string   `json:"key"`
			Label       string   `json:"label"`
			Score       int      `json:"score"`
			Summary     string   `json:"summary"`
			Suggestions []string `json:"suggestions"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.EvaluationID == "" {
		t.Error("evaluation_id must not be empty")
	}
	if env.CreatedAt.IsZero() {
		t.Error("created_at must not be zero")
	}
	if env.OverallScore < 0 || env.OverallScore > 100 {
		t.Errorf("overall_score=%d out of [0,100]", env.OverallScore)
	}
	if env.OverallSummary == "" {
		t.Error("overall_summary must not be empty")
	}
	if env.CohesionScore < 0 || env.CohesionScore > 100 {
		t.Errorf("cohesion_score=%d out of [0,100]", env.CohesionScore)
	}
	if env.CohesionSummary == "" {
		t.Error("cohesion_summary must not be empty")
	}
	if env.CohesionSuggestions == nil {
		t.Error("cohesion_suggestions must be a JSON array (not null)")
	}

	wantSectionKeys := []string{"title", "initial_prompt", "priority", "structure"}
	if len(env.Sections) != len(wantSectionKeys) {
		t.Fatalf("sections len=%d want %d", len(env.Sections), len(wantSectionKeys))
	}
	for i, want := range wantSectionKeys {
		got := env.Sections[i]
		if got.Key != want {
			t.Errorf("sections[%d].key=%q want %q (fixed order docs/API-HTTP.md)", i, got.Key, want)
		}
		if got.Label == "" {
			t.Errorf("sections[%d].label empty", i)
		}
		if got.Score < 0 || got.Score > 100 {
			t.Errorf("sections[%d].score=%d out of [0,100]", i, got.Score)
		}
		if got.Summary == "" {
			t.Errorf("sections[%d].summary empty", i)
		}
		if got.Suggestions == nil {
			t.Errorf("sections[%d].suggestions must be a JSON array (not null)", i)
		}
	}
}

// TestHTTP_evaluateDraft_emptyBody pins the documented "all fields optional"
// behavior: posting `{}` still returns a full 201 envelope. The score values
// don't matter — what matters is that the contract surface stays stable for
// the minimum viable request shape the web client may send.
func TestHTTP_evaluateDraft_emptyBody(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d (want 201) body=%s", res.StatusCode, raw)
	}
	var env struct {
		EvaluationID string `json:"evaluation_id"`
		Sections     []struct {
			Key string `json:"key"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if env.EvaluationID == "" {
		t.Fatal("evaluation_id must be set even for empty body request")
	}
	if len(env.Sections) != 4 {
		t.Fatalf("sections len=%d want 4", len(env.Sections))
	}
}

// TestHTTP_evaluateDraft_invalidTaskTypeReturns400 pins the documented 400
// for invalid `task_type` (the only required-shape validation the store
// performs after task_type defaulting in EvaluateDraftTask).
func TestHTTP_evaluateDraft_invalidTaskTypeReturns400(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json",
		strings.NewReader(`{"title":"x","task_type":"not-a-real-type"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d (want 400) body=%s", res.StatusCode, body)
	}
}

// TestHTTP_evaluateDraft_doesNotPublishSSE pins the SSE invariant called
// out in docs/API-SSE.md: the draft scorer never publishes — `task_updated`
// only fires once `POST /tasks` actually creates the underlying row. This
// guards against a future regression where evaluate accidentally calls
// notifyChange.
func TestHTTP_evaluateDraft_doesNotPublishSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()
	ch, cancel := hub.Subscribe()
	defer cancel()

	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json",
		strings.NewReader(`{"title":"sse-evaluate","priority":"high"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("evaluate status %d (want 201)", res.StatusCode)
	}

	got := summarize(drainSSE(t, ch, 1, 200*time.Millisecond))
	if len(got) != 0 {
		t.Fatalf("/tasks/evaluate published SSE events unexpectedly: %v", got)
	}
}
