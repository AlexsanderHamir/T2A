package handler

// GET /task-drafts (list) contract pins. Split out of
// handler_http_drafts_contract_test.go in Session 35 (P6 — file size).

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

func listDrafts(t *testing.T, baseURL, query string) (*http.Response, []byte) {
	t.Helper()
	url := baseURL + "/task-drafts"
	if query != "" {
		url += "?" + query
	}
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_listDrafts_envelope pins the GET /task-drafts envelope contract:
// `{"drafts":[...]}` always present, drafts is always a JSON array (`[]`
// when empty, never null/omitted), each row exact key set without payload,
// ordering is updated_at DESC.
func TestHTTP_listDrafts_envelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	t.Run("emptyDB_returnsEmptyArray", func(t *testing.T) {
		res, raw := listDrafts(t, srv.URL, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
		}
		// Raw-bytes guard: a future drift to `null` or omitting the key would
		// silently break the SPA (`drafts.map(...)` on null throws).
		if !strings.Contains(string(raw), `"drafts":[]`) {
			t.Fatalf("body=%s want literal `\"drafts\":[]` substring (docs claim drafts is always a JSON array, [] when empty)", raw)
		}
	})

	t.Run("populated_orderingAndKeys", func(t *testing.T) {
		// Three drafts saved in order alpha → beta → gamma; the GET response
		// must return them gamma → beta → alpha (updated_at DESC). A 10ms
		// sleep between saves keeps the timestamps strictly monotonic across
		// SQLite's millisecond resolution.
		ids := make([]string, 3)
		for i, n := range []string{"alpha", "beta", "gamma"} {
			res, raw := saveDraft(t, srv.URL, `{"name":"`+n+`","payload":{}}`)
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("seed %s status %d body=%s", n, res.StatusCode, raw)
			}
			var sum struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &sum); err != nil {
				t.Fatalf("decode seed: %v", err)
			}
			ids[i] = sum.ID
			time.Sleep(10 * time.Millisecond)
		}

		res, raw := listDrafts(t, srv.URL, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("list status %d body=%s", res.StatusCode, raw)
		}
		var env struct {
			Drafts []map[string]json.RawMessage `json:"drafts"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("decode envelope: %v body=%s", err, raw)
		}
		if len(env.Drafts) != 3 {
			t.Fatalf("len(drafts)=%d want 3 body=%s", len(env.Drafts), raw)
		}
		wantKeys := []string{"id", "name", "created_at", "updated_at"}
		sort.Strings(wantKeys)
		for i, row := range env.Drafts {
			gotKeys := make([]string, 0, len(row))
			for k := range row {
				gotKeys = append(gotKeys, k)
			}
			sort.Strings(gotKeys)
			if !equalStringSlices(gotKeys, wantKeys) {
				t.Fatalf("drafts[%d] keys=%v want %v (no payload on summary view)", i, gotKeys, wantKeys)
			}
		}
		// Order: gamma (newest) → beta → alpha (oldest).
		var first, second, third struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(raw, &struct {
			Drafts []*struct {
				Name string `json:"name"`
			} `json:"drafts"`
		}{Drafts: []*struct {
			Name string `json:"name"`
		}{&first, &second, &third}})
		if first.Name != "gamma" || second.Name != "beta" || third.Name != "alpha" {
			t.Fatalf("ordering=%q,%q,%q want gamma,beta,alpha (updated_at DESC)",
				first.Name, second.Name, third.Name)
		}
	})
}

// TestHTTP_listDrafts_400Limit pins the documented bare 400 wire phrases for
// the limit query parameter. The handler emits its own messages here (not
// the store's invalidInputDetail path), so the wording is asserted verbatim.
func TestHTTP_listDrafts_400Limit(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"overlongValue", "limit=" + strings.Repeat("1", maxListIntQueryParamBytes+1), "limit value too long"},
		{"nonNumeric", "limit=nope", "limit must be integer 0..100"},
		{"negative", "limit=-1", "limit must be integer 0..100"},
		{"overMax", "limit=101", "limit must be integer 0..100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, raw := listDrafts(t, srv.URL, tc.query)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d (want 400) body=%s", res.StatusCode, raw)
			}
			var errBody jsonErrorBody
			if err := json.Unmarshal(raw, &errBody); err != nil {
				t.Fatalf("decode: %v body=%s", err, raw)
			}
			if errBody.Error != tc.want {
				t.Fatalf("error=%q want %q (docs/API-HTTP.md /task-drafts/* 400 strings)", errBody.Error, tc.want)
			}
		})
	}
}
