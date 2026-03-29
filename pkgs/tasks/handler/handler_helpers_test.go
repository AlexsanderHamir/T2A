package handler

import (
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
	if err := decodeJSON(strings.NewReader(string(b)), &got); err != nil {
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
	if err := decodeJSON(strings.NewReader(string(b)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status == nil || *got.Status != domain.StatusRunning {
		t.Fatalf("status: %v", got.Status)
	}
}

func TestDecodeJSON_rejectsUnknownField(t *testing.T) {
	const raw = `{"title":"x","initial_prompt":"","nope":1}`
	var got taskCreateJSON
	err := decodeJSON(strings.NewReader(raw), &got)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeJSON_rejectsTrailingJSON(t *testing.T) {
	const raw = `{"title":"x","initial_prompt":""}{}`
	var got taskCreateJSON
	err := decodeJSON(strings.NewReader(raw), &got)
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
		{name: "internal", err: errors.New("db unavailable"), wantCode: http.StatusInternalServerError, wantMsg: "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg := storeErrHTTPResponse(tt.err)
			if code != tt.wantCode || msg != tt.wantMsg {
				t.Fatalf("code=%d msg=%q want code=%d msg=%q", code, msg, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestParseListParams(t *testing.T) {
	tests := []struct {
		name       string
		q          url.Values
		wantLimit  int
		wantOffset int
		wantErr    bool
	}{
		{name: "defaults", q: url.Values{}, wantLimit: 50, wantOffset: 0},
		{name: "limit_200_offset_3", q: url.Values{"limit": {"200"}, "offset": {"3"}}, wantLimit: 200, wantOffset: 3},
		{name: "limit_nan", q: url.Values{"limit": {"nope"}}, wantErr: true},
		{name: "limit_201", q: url.Values{"limit": {"201"}}, wantErr: true},
		{name: "offset_negative", q: url.Values{"offset": {"-1"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset, err := parseListParams(tt.q)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("limit=%d offset=%d want limit=%d offset=%d", limit, offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

func TestDecodeJSON_trailing_garbage_after_object(t *testing.T) {
	const raw = `{"title":"x","initial_prompt":""}junk`
	var got taskCreateJSON
	err := decodeJSON(strings.NewReader(raw), &got)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLogRequestFailure_warnAndError(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	logRequestFailure("test.op", errors.New("client"), http.StatusBadRequest)
	logRequestFailure("test.op", errors.New("server"), http.StatusInternalServerError)
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
