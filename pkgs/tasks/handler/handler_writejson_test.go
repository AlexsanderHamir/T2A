package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	var body jsonErrorBody
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v body=%q", err, rr.Body.String())
	}
	const want = "internal server error" // handler_http_json.go::writeJSON encode-fallback → writeJSONError
	if body.Error != want {
		t.Fatalf("error: got %q want %q (same bare phrase as stack_test.go recovery path)", body.Error, want)
	}
}
