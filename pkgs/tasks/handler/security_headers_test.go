package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/httpsecurityexpect"
	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestStreamEvents_sets_security_headers(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	h := &Handler{store: store.NewStore(db), hub: NewSSEHub(), repoProv: NewStaticRepoProvider(nil)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.streamEvents(rec, req)
	httpsecurityexpect.AssertBaselineHeaders(t, rec.Header())
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
}
