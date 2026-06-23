package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestHealthReady_schemaPendingReturns503(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil,
		WithGitAvailable(true),
		WithSchemaDriftReport(postgres.SchemaDriftReport{
			Status:       postgres.SchemaDriftPending,
			CodeRevision: postgres.SchemaRevision,
			DBRevision:   0,
		}))

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	checks, _ := body["checks"].(map[string]any)
	if checks["schema"] != "pending" {
		t.Fatalf("checks.schema=%v want pending", checks["schema"])
	}
	schema, ok := body["schema"].(map[string]any)
	if !ok {
		t.Fatalf("schema block missing: %v", body)
	}
	msg, _ := schema["message"].(string)
	if msg == "" {
		t.Fatalf("schema.message missing: %v", schema)
	}
	if !strings.Contains(msg, "migrate") {
		t.Fatalf("schema.message=%q", msg)
	}
}

func TestHealthReady_schemaOKWhenAligned(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil,
		WithGitAvailable(true),
		WithSchemaDriftReport(postgres.SchemaDriftReport{
			Status:       postgres.SchemaDriftOK,
			CodeRevision: postgres.SchemaRevision,
			DBRevision:   postgres.SchemaRevision,
		}))

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
