package handler

// GET /task-drafts/{id} contract pins. Split out of
// handler_http_drafts_contract_test.go in Session 35 (P6 — file size).

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"testing"
)

func getDraft(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(baseURL + "/task-drafts/" + id)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_getDraft_envelope pins the GET /task-drafts/{id} 200 envelope
// shape (exact five keys) and the always-present payload invariant.
func TestHTTP_getDraft_envelope(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	resSave, rawSave := saveDraft(t, srv.URL, `{"name":"detail","payload":{"k":"v"}}`)
	if resSave.StatusCode != http.StatusCreated {
		t.Fatalf("seed save status %d body=%s", resSave.StatusCode, rawSave)
	}
	var sum struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rawSave, &sum)

	res, raw := getDraft(t, srv.URL, sum.ID)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200) body=%s", res.StatusCode, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	wantKeys := []string{"id", "name", "payload", "created_at", "updated_at"}
	gotKeys := make([]string, 0, len(top))
	for k := range top {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if !equalStringSlices(gotKeys, wantKeys) {
		t.Fatalf("envelope keys=%v want %v (docs/API-HTTP.md GET /task-drafts/{id} pins {id,name,payload,created_at,updated_at})", gotKeys, wantKeys)
	}
	if string(top["payload"]) == "" || string(top["payload"]) == "null" {
		t.Fatalf("payload=%q want non-empty JSON value (docs claim payload is always present)", top["payload"])
	}
}

// TestHTTP_getDraft_404 pins the documented 404 mapping for an unknown id.
func TestHTTP_getDraft_404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, raw := getDraft(t, srv.URL, "no-such-draft-9999")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d (want 404) body=%s", res.StatusCode, raw)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Error != "not found" {
		t.Fatalf("error=%q want %q (store-error mapping)", errBody.Error, "not found")
	}
}
