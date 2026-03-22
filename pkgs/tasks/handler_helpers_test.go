package tasks

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if got.Status != StatusReady {
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
	if got.Status == nil || *got.Status != StatusRunning {
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
	if !errors.Is(err, ErrInvalidInput) && !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestParseListParams_defaults(t *testing.T) {
	limit, offset, err := parseListParams(url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if limit != 50 || offset != 0 {
		t.Fatalf("limit=%d offset=%d", limit, offset)
	}
}

func TestParseListParams_invalidLimit(t *testing.T) {
	_, _, err := parseListParams(url.Values{"limit": {"nope"}})
	if err == nil {
		t.Fatal("expected error")
	}
	_, _, err = parseListParams(url.Values{"limit": {"201"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseListParams_invalidOffset(t *testing.T) {
	_, _, err := parseListParams(url.Values{"offset": {"-1"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestActorFromRequest(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if actorFromRequest(r) != ActorUser {
		t.Fatal("default actor")
	}
	r.Header.Set("X-Actor", "agent")
	if actorFromRequest(r) != ActorAgent {
		t.Fatal("agent")
	}
}
