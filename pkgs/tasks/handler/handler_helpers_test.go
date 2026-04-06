package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestDecodeJSON_taskCreate_fixture(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "task_create.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got taskCreateJSON
	if err := decodeJSON(context.Background(), strings.NewReader(string(b)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Example from testdata" {
		t.Fatalf("title: %q", got.Title)
	}
	if got.Status != domain.StatusReady {
		t.Fatalf("status: %s", got.Status)
	}
}

func TestDecodeJSON_taskPatch_fixture(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "task_patch.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got taskPatchJSON
	if err := decodeJSON(context.Background(), strings.NewReader(string(b)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status == nil || *got.Status != domain.StatusRunning {
		t.Fatalf("status: %v", got.Status)
	}
}

func TestDecodeJSON_rejectsUnknownField(t *testing.T) {
	const raw = `{"title":"x","initial_prompt":"","nope":1}`
	var got taskCreateJSON
	err := decodeJSON(context.Background(), strings.NewReader(raw), &got)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeJSON_rejectsTrailingJSON(t *testing.T) {
	const raw = `{"title":"x","initial_prompt":""}{}`
	var got taskCreateJSON
	err := decodeJSON(context.Background(), strings.NewReader(raw), &got)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrInvalidInput) && !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestStoreErrHTTPResponse(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{name: "not_found", err: domain.ErrNotFound, wantCode: http.StatusNotFound, wantMsg: "not found"},
		{name: "invalid_input_plain", err: domain.ErrInvalidInput, wantCode: http.StatusBadRequest, wantMsg: "bad request"},
		{
			name:     "invalid_input_detail",
			err:      fmt.Errorf("%w: empty id", domain.ErrInvalidInput),
			wantCode: http.StatusBadRequest,
			wantMsg:  "empty id",
		},
		{name: "deadline_exceeded", err: context.DeadlineExceeded, wantCode: http.StatusGatewayTimeout, wantMsg: "request timed out"},
		{name: "context_canceled", err: context.Canceled, wantCode: http.StatusRequestTimeout, wantMsg: "request canceled"},
		{name: "conflict", err: domain.ErrConflict, wantCode: http.StatusConflict, wantMsg: "task id already exists"},
		{name: "internal", err: errors.New("db unavailable"), wantCode: http.StatusInternalServerError, wantMsg: "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg := storeErrHTTPResponse(context.Background(), tt.err)
			if code != tt.wantCode || msg != tt.wantMsg {
				t.Fatalf("code=%d msg=%q want code=%d msg=%q", code, msg, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestParseListParams(t *testing.T) {
	id2 := "22222222-2222-4222-8222-222222222222"
	tests := []struct {
		name        string
		q           url.Values
		wantLimit   int
		wantOffset  int
		wantAfterID string
		wantErr     bool
	}{
		{name: "defaults", q: url.Values{}, wantLimit: 50, wantOffset: 0},
		{name: "limit_200_offset_3", q: url.Values{"limit": {"200"}, "offset": {"3"}}, wantLimit: 200, wantOffset: 3},
		{name: "after_id_only", q: url.Values{"after_id": {id2}, "limit": {"10"}}, wantLimit: 10, wantOffset: 0, wantAfterID: id2},
		{name: "after_id_with_offset", q: url.Values{"after_id": {id2}, "offset": {"0"}}, wantErr: true},
		{name: "after_id_bad_uuid", q: url.Values{"after_id": {"not-a-uuid"}}, wantErr: true},
		{name: "limit_nan", q: url.Values{"limit": {"nope"}}, wantErr: true},
		{name: "limit_201", q: url.Values{"limit": {"201"}}, wantErr: true},
		{name: "offset_negative", q: url.Values{"offset": {"-1"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset, afterID, err := parseListParams(context.Background(), tt.q)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if limit != tt.wantLimit || offset != tt.wantOffset || afterID != tt.wantAfterID {
				t.Fatalf("limit=%d offset=%d afterID=%q want limit=%d offset=%d afterID=%q", limit, offset, afterID, tt.wantLimit, tt.wantOffset, tt.wantAfterID)
			}
		})
	}
}

func TestDecodeJSON_trailing_garbage_after_object(t *testing.T) {
	const raw = `{"title":"x","initial_prompt":""}junk`
	var got taskCreateJSON
	err := decodeJSON(context.Background(), strings.NewReader(raw), &got)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLogRequestFailure_warnAndError(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	logRequestFailure(context.Background(), "test.op", errors.New("client"), http.StatusBadRequest)
	logRequestFailure(context.Background(), "test.op", errors.New("server"), http.StatusInternalServerError)
}

func TestActorFromRequest(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if actorFromRequest(r) != domain.ActorUser {
		t.Fatal("default actor")
	}
	r.Header.Set("X-Actor", "agent")
	if actorFromRequest(r) != domain.ActorAgent {
		t.Fatal("agent")
	}
}
