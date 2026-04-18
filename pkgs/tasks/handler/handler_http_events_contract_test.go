package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestHTTP_taskEvents_fullListShape pins the documented full-list response
// shape (no paging params) from docs/API-HTTP.md: ascending seq, no
// `limit`/`total`/`range_*` keys (omitempty pointers stay omitted), but
// `has_more_newer`/`has_more_older` are present and false, and
// `approval_pending` is always present. Each row carries `seq`, `at`,
// `type`, `by`, and `data` (defaulted to "{}" when the underlying event
// has no payload).
func TestHTTP_taskEvents_fullListShape(t *testing.T) {
	srv, _ := newTaskTestServerWithStore(t)
	defer srv.Close()
	created := postTaskJSON(t, srv, `{"title":"contract","priority":"medium"}`, http.StatusCreated)

	res, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}

	wantKeys := []string{"task_id", "events", "has_more_newer", "has_more_older", "approval_pending"}
	for _, k := range wantKeys {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q in full-list response (docs/API-HTTP.md): %s", k, body)
		}
	}
	for _, k := range []string{"limit", "total", "range_start", "range_end"} {
		if _, ok := raw[k]; ok {
			t.Errorf("full-list response unexpectedly includes %q (should be omitted): %s", k, body)
		}
	}

	var hasMoreNewer, hasMoreOlder, approvalPending bool
	if err := json.Unmarshal(raw["has_more_newer"], &hasMoreNewer); err != nil || hasMoreNewer {
		t.Errorf("has_more_newer want false, got raw=%s err=%v", raw["has_more_newer"], err)
	}
	if err := json.Unmarshal(raw["has_more_older"], &hasMoreOlder); err != nil || hasMoreOlder {
		t.Errorf("has_more_older want false, got raw=%s err=%v", raw["has_more_older"], err)
	}
	if err := json.Unmarshal(raw["approval_pending"], &approvalPending); err != nil || approvalPending {
		t.Errorf("approval_pending want false, got raw=%s err=%v", raw["approval_pending"], err)
	}

	var events []map[string]json.RawMessage
	if err := json.Unmarshal(raw["events"], &events); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(events) < 1 {
		t.Fatalf("want >=1 event row, got %d", len(events))
	}
	for i, ev := range events {
		for _, k := range []string{"seq", "at", "type", "by", "data"} {
			if _, ok := ev[k]; !ok {
				t.Errorf("event[%d] missing required key %q: %v", i, k, ev)
			}
		}
		if string(ev["data"]) == "" || string(ev["data"]) == "null" {
			t.Errorf("event[%d] data must not be null/empty (defaults to {}): got %q", i, ev["data"])
		}
	}
}

// TestHTTP_taskEvents_cursorShape pins the cursor-paged shape: descending
// seq, includes `limit`, `total`, `range_start`, `range_end`,
// `has_more_newer`, `has_more_older`, and `approval_pending`. Empty pages
// must still send `limit`/`total` but omit `range_start`/`range_end`.
func TestHTTP_taskEvents_cursorShape(t *testing.T) {
	srv, _ := newTaskTestServerWithStore(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"c","priority":"medium"}`, http.StatusCreated)
	patchTaskJSON(t, srv, task.ID, `{"title":"c2"}`, http.StatusOK)

	res, err := http.Get(srv.URL + "/tasks/" + task.ID + "/events?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, body)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"task_id", "events", "limit", "total", "range_start", "range_end", "has_more_newer", "has_more_older", "approval_pending"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q in cursor-paged response: %s", k, body)
		}
	}

	var events []struct {
		Seq int64 `json:"seq"`
	}
	if err := json.Unmarshal(raw["events"], &events); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events on cursor page, got %d", len(events))
	}
	if events[0].Seq <= events[1].Seq {
		t.Fatalf("cursor page must be descending seq, got %d then %d", events[0].Seq, events[1].Seq)
	}

	resAfter, err := http.Get(srv.URL + "/tasks/" + task.ID + "/events?after_seq=999999")
	if err != nil {
		t.Fatal(err)
	}
	defer resAfter.Body.Close()
	bodyAfter, _ := io.ReadAll(resAfter.Body)
	if resAfter.StatusCode != http.StatusOK {
		t.Fatalf("after_seq status %d body=%s", resAfter.StatusCode, bodyAfter)
	}
	var rawAfter map[string]json.RawMessage
	if err := json.Unmarshal(bodyAfter, &rawAfter); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"limit", "total"} {
		if _, ok := rawAfter[k]; !ok {
			t.Errorf("empty cursor page must still include %q: %s", k, bodyAfter)
		}
	}
	for _, k := range []string{"range_start", "range_end"} {
		if _, ok := rawAfter[k]; ok {
			t.Errorf("empty cursor page must omit %q: %s", k, bodyAfter)
		}
	}
}

// TestHTTP_taskEvents_orderingFullVsCursor ensures the documented ordering
// contract is real: full-list is ascending, cursor-paged is descending.
func TestHTTP_taskEvents_orderingFullVsCursor(t *testing.T) {
	srv, _ := newTaskTestServerWithStore(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"o","priority":"medium"}`, http.StatusCreated)
	for i := 0; i < 3; i++ {
		patchTaskJSON(t, srv, task.ID, `{"title":"o-`+string(rune('a'+i))+`"}`, http.StatusOK)
	}

	resFull, err := http.Get(srv.URL + "/tasks/" + task.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resFull.Body.Close()
	var fullPayload struct {
		Events []struct {
			Seq int64 `json:"seq"`
		} `json:"events"`
	}
	if err := json.NewDecoder(resFull.Body).Decode(&fullPayload); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(fullPayload.Events); i++ {
		if fullPayload.Events[i].Seq <= fullPayload.Events[i-1].Seq {
			t.Fatalf("full-list must be ascending; seq[%d]=%d after seq[%d]=%d", i, fullPayload.Events[i].Seq, i-1, fullPayload.Events[i-1].Seq)
		}
	}

	resCur, err := http.Get(srv.URL + "/tasks/" + task.ID + "/events?limit=200")
	if err != nil {
		t.Fatal(err)
	}
	defer resCur.Body.Close()
	var curPayload struct {
		Events []struct {
			Seq int64 `json:"seq"`
		} `json:"events"`
	}
	if err := json.NewDecoder(resCur.Body).Decode(&curPayload); err != nil {
		t.Fatal(err)
	}
	if len(curPayload.Events) != len(fullPayload.Events) {
		t.Fatalf("full vs cursor count differ: full=%d cursor=%d", len(fullPayload.Events), len(curPayload.Events))
	}
	for i := 1; i < len(curPayload.Events); i++ {
		if curPayload.Events[i].Seq >= curPayload.Events[i-1].Seq {
			t.Fatalf("cursor page must be descending; seq[%d]=%d after seq[%d]=%d", i, curPayload.Events[i].Seq, i-1, curPayload.Events[i-1].Seq)
		}
	}
}

// TestHTTP_taskEvents_validation400s pins every documented 400 string for
// the events endpoints. If the wording or set of triggers drifts, this
// test fails so docs/API-HTTP.md is updated in the same PR.
func TestHTTP_taskEvents_validation400s(t *testing.T) {
	srv, _ := newTaskTestServerWithStore(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"v","priority":"medium"}`, http.StatusCreated)

	long := strings.Repeat("9", 33)
	cases := []struct {
		name, url, wantSubstr string
	}{
		{"offset rejected", "/tasks/" + task.ID + "/events?offset=0", "offset is not supported for task events"},
		{"both cursors set", "/tasks/" + task.ID + "/events?before_seq=1&after_seq=1", "before_seq and after_seq cannot both be set"},
		{"before_seq too long", "/tasks/" + task.ID + "/events?before_seq=" + long, "before_seq or after_seq too long"},
		{"after_seq too long", "/tasks/" + task.ID + "/events?after_seq=" + long, "before_seq or after_seq too long"},
		{"limit too long", "/tasks/" + task.ID + "/events?limit=" + long, "limit too long"},
		{"limit out of range", "/tasks/" + task.ID + "/events?limit=999", "limit must be integer 0..200"},
		{"limit non-numeric", "/tasks/" + task.ID + "/events?limit=nope", "limit must be integer 0..200"},
		{"before_seq zero", "/tasks/" + task.ID + "/events?before_seq=0", "before_seq must be a positive integer"},
		{"after_seq zero", "/tasks/" + task.ID + "/events?after_seq=0", "after_seq must be a positive integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := http.Get(srv.URL + tc.url)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, body)
			}
			if !strings.Contains(string(body), tc.wantSubstr) {
				t.Fatalf("error body must contain %q (docs/API-HTTP.md), got %s", tc.wantSubstr, body)
			}
		})
	}
}

// TestHTTP_taskEvents_seqPathValidation pins the {seq} path-segment 400s
// for both GET and PATCH event-detail routes.
func TestHTTP_taskEvents_seqPathValidation(t *testing.T) {
	srv, _ := newTaskTestServerWithStore(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"s","priority":"medium"}`, http.StatusCreated)

	long := strings.Repeat("9", 33)
	cases := []struct {
		name, method, url, wantSubstr string
	}{
		{"GET seq too long", http.MethodGet, "/tasks/" + task.ID + "/events/" + long, "seq too long"},
		{"GET seq zero", http.MethodGet, "/tasks/" + task.ID + "/events/0", "seq must be a positive integer"},
		{"GET seq non-numeric", http.MethodGet, "/tasks/" + task.ID + "/events/nope", "seq must be a positive integer"},
		{"PATCH seq too long", http.MethodPatch, "/tasks/" + task.ID + "/events/" + long, "seq too long"},
		{"PATCH seq zero", http.MethodPatch, "/tasks/" + task.ID + "/events/0", "seq must be a positive integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rdr io.Reader
			if tc.method == http.MethodPatch {
				rdr = strings.NewReader(`{"user_response":"x"}`)
			}
			req, err := http.NewRequest(tc.method, srv.URL+tc.url, rdr)
			if err != nil {
				t.Fatal(err)
			}
			if tc.method == http.MethodPatch {
				req.Header.Set("Content-Type", "application/json")
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, body)
			}
			if !strings.Contains(string(body), tc.wantSubstr) {
				t.Fatalf("error body must contain %q, got %s", tc.wantSubstr, body)
			}
		})
	}
}

// TestHTTP_taskEvents_404OnUnknownTask pins the documented 404 when the
// task does not exist (handler must Get(id) before pagination work).
func TestHTTP_taskEvents_404OnUnknownTask(t *testing.T) {
	srv, _ := newTaskTestServerWithStore(t)
	defer srv.Close()

	missingID := "00000000-0000-4000-8000-000000000000"
	for _, url := range []string{
		"/tasks/" + missingID + "/events",
		"/tasks/" + missingID + "/events?limit=5",
		"/tasks/" + missingID + "/events/1",
	} {
		res, err := http.Get(srv.URL + url)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d (want 404) body=%s", url, res.StatusCode, body)
		}
	}
}

// TestHTTP_taskEvents_patchValidation pins PATCH validation: empty body,
// empty user_response, oversize user_response (>10 000 bytes), and
// non-accepting event types all return 400; missing event returns 404.
func TestHTTP_taskEvents_patchValidation(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	task := postTaskJSON(t, srv, `{"title":"p","priority":"medium"}`, http.StatusCreated)

	patch := func(seq int, body, actor string) (int, string) {
		req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+task.ID+"/events/"+strconv.Itoa(seq), strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if actor != "" {
			req.Header.Set("X-Actor", actor)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}

	// seq=1 is task_created — does not accept user response.
	if code, body := patch(1, `{"user_response":"hi"}`, ""); code != http.StatusBadRequest {
		t.Errorf("non-accepting event type want 400, got %d body=%s", code, body)
	}

	// Append approval_requested at seq=2 so the next checks have a real target.
	if err := st.AppendTaskEvent(context.Background(), task.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	if code, body := patch(2, `{"user_response":"   "}`, ""); code != http.StatusBadRequest {
		t.Errorf("empty (whitespace) user_response want 400, got %d body=%s", code, body)
	}

	tooLong := `{"user_response":"` + strings.Repeat("a", 10_001) + `"}`
	if code, body := patch(2, tooLong, ""); code != http.StatusBadRequest {
		t.Errorf("oversize user_response want 400, got %d body=%s", code, body)
	}

	if code, body := patch(99, `{"user_response":"hi"}`, ""); code != http.StatusNotFound {
		t.Errorf("missing event seq want 404, got %d body=%s", code, body)
	}
}
