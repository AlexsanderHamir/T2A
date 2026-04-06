package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON_encodeFailureReturns500JSON(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	bad := map[string]any{"x": make(chan int)}
	writeJSON(rr, req, "test.write_json", http.StatusOK, bad)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d body=%q", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "internal server error") {
		t.Fatalf("body should mention internal error: %q", rr.Body.String())
	}
}
