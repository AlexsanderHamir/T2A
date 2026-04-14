package apijson

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

func TestWriteJSONError_includesRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(logctx.ContextWithRequestID(req.Context(), "rid-apijson-1"))
	WriteJSONError(rec, req, "apijson.test", http.StatusTeapot, "short and stout", nil)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("code %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type %q", ct)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("expected security header")
	}
	var out struct {
		Error     string `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "short and stout" || out.RequestID != "rid-apijson-1" {
		t.Fatalf("got %+v", out)
	}
}

func TestWriteJSONError_nilRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSONError(rec, nil, "apijson.nilreq", http.StatusBadRequest, "bad", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code %d", rec.Code)
	}
}
