package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

func TestResolveAndAttachRequestID_preservesExistingContextID(t *testing.T) {
	const want = "existing-req-id"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	ctx := logctx.ContextWithRequestID(req.Context(), want)
	req = req.WithContext(ctx)

	out := resolveAndAttachRequestID(rec, req)
	if got := logctx.RequestIDFromContext(out.Context()); got != want {
		t.Fatalf("context request_id: got %q want %q", got, want)
	}
	if got := rec.Header().Get("X-Request-ID"); got != want {
		t.Fatalf("response header X-Request-ID: got %q want %q", got, want)
	}
}
