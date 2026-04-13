package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestWriteJSONError_includes_request_id_from_context(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ContextWithRequestID(req.Context(), "unit-rid-1"))
	writeJSONError(rec, req, "test.op", http.StatusBadRequest, "bad")
	var out struct {
		Error     string `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "bad" || out.RequestID != "unit-rid-1" {
		t.Fatalf("got %+v", out)
	}
}

func TestHTTP_error_JSON_includes_request_id_with_access_middleware(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	api := WithAccessLog(NewHandler(store.NewStore(db), NewSSEHub(), nil))
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"unknown_field":true}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
	ridHeader := strings.TrimSpace(res.Header.Get("X-Request-ID"))
	if ridHeader == "" {
		t.Fatal("missing X-Request-ID on response")
	}
	var out struct {
		Error     string `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.RequestID != ridHeader {
		t.Fatalf("request_id %q want %q", out.RequestID, ridHeader)
	}
}
