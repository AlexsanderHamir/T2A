package handler

// DELETE /task-drafts/{id} contract pins. Split out of
// handler_http_drafts_contract_test.go in Session 35 (P6 — file size).

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func deleteDraft(t *testing.T, baseURL, id string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/task-drafts/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, raw
}

// TestHTTP_deleteDraft_204AndIdempotent404 pins the 204 + empty-body success
// contract and the documented "re-DELETE returns 404" behavior.
func TestHTTP_deleteDraft_204AndIdempotent404(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	resSave, rawSave := saveDraft(t, srv.URL, `{"name":"to-delete","payload":{}}`)
	if resSave.StatusCode != http.StatusCreated {
		t.Fatalf("seed save status %d body=%s", resSave.StatusCode, rawSave)
	}
	var sum struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rawSave, &sum)

	res1, raw1 := deleteDraft(t, srv.URL, sum.ID)
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first delete status %d (want 204) body=%q", res1.StatusCode, raw1)
	}
	if len(raw1) != 0 {
		t.Fatalf("first delete body=%q want empty (docs claim 204 + empty body)", raw1)
	}

	res2, raw2 := deleteDraft(t, srv.URL, sum.ID)
	if res2.StatusCode != http.StatusNotFound {
		t.Fatalf("re-delete status %d (want 404) body=%s", res2.StatusCode, raw2)
	}
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw2, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw2)
	}
	if errBody.Error != "not found" {
		t.Fatalf("re-delete error=%q want %q", errBody.Error, "not found")
	}
}
