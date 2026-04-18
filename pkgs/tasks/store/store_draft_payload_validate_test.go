package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestStore_SaveDraft_payload_normalizes_null pins the documented invariant
// for `task_drafts.payload_json` (docs/API-HTTP.md POST /task-drafts: "a
// missing or null payload is silently coerced to {} so a follow-up GET always
// returns a JSON object"). The store must collapse the JSON literal "null"
// (and whitespace-only payloads) to canonical "{}" rather than persisting the
// literal "null", which would round-trip on GET /task-drafts/{id} as
// `"payload": null` — a contract violation that breaks downstream parsers
// that assume an object.
func TestStore_SaveDraft_payload_normalizes_null(t *testing.T) {
	s, ctx := newDraftStoreVal(t)

	cases := []struct {
		name    string
		payload []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"whitespace", []byte("   ")},
		{"json_null", []byte("null")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved, err := s.SaveDraft(ctx, "", "n-"+tc.name, tc.payload)
			if err != nil {
				t.Fatalf("save draft: %v", err)
			}
			got, err := s.GetDraft(ctx, saved.ID)
			if err != nil {
				t.Fatalf("get draft: %v", err)
			}
			if string(got.Payload) != "{}" {
				t.Fatalf("payload = %q, want {} (must normalize per documented invariant)", string(got.Payload))
			}
		})
	}
}

// TestStore_SaveDraft_payload_rejects_non_object_json asserts that payloads
// that are syntactically valid JSON but not objects (string, number, array,
// bool) are rejected with domain.ErrInvalidInput so the handler surfaces a
// 400. The documented contract is that `GET /task-drafts/{id}` always returns
// `payload` as a JSON object; silent coercion (or pass-through) would store
// shapes that violate that promise.
func TestStore_SaveDraft_payload_rejects_non_object_json(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"string", []byte(`"foo"`)},
		{"number", []byte(`123`)},
		{"array", []byte(`[1,2,3]`)},
		{"bool", []byte(`true`)},
		{"malformed", []byte(`{not-json`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx := newDraftStoreVal(t)
			_, err := s.SaveDraft(ctx, "", "n", tc.payload)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput for payload=%s", err, string(tc.payload))
			}
		})
	}
}

// TestStore_SaveDraft_payload_passes_through_object pins the happy path so
// the new validation cannot regress into rejecting valid object payloads.
func TestStore_SaveDraft_payload_passes_through_object(t *testing.T) {
	s, ctx := newDraftStoreVal(t)
	body := []byte(`{"title":"draft","priority":"medium"}`)
	saved, err := s.SaveDraft(ctx, "", "n", body)
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	got, err := s.GetDraft(ctx, saved.ID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if string(got.Payload) != string(body) {
		t.Fatalf("payload = %q, want %q", string(got.Payload), string(body))
	}
}

// TestStore_SaveDraft_payload_repeat_normalizes_on_replace covers the upsert
// path — a second SaveDraft with the same id and a now-null payload must
// also normalize to "{}" (the docs explicitly call out replace semantics).
func TestStore_SaveDraft_payload_repeat_normalizes_on_replace(t *testing.T) {
	s, ctx := newDraftStoreVal(t)
	first, err := s.SaveDraft(ctx, "", "n", []byte(`{"k":1}`))
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if _, err := s.SaveDraft(ctx, first.ID, "n", []byte("null")); err != nil {
		t.Fatalf("replace save: %v", err)
	}
	got, err := s.GetDraft(ctx, first.ID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if string(got.Payload) != "{}" {
		t.Fatalf("payload after replace = %q, want {}", string(got.Payload))
	}
}

func newDraftStoreVal(t *testing.T) (*Store, context.Context) {
	t.Helper()
	return NewStore(tasktestdb.OpenSQLite(t)), context.Background()
}
