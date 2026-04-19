package kernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestNormalizeJSONObject_trimsLeadingAndTrailingWhitespace pins the
// invariant that the on-disk byte shape of normalized JSON object
// payloads (drafts.payload_json, cycles.metadata_json, etc.) never
// carries leading or trailing whitespace, regardless of which input
// branch fires inside NormalizeJSONObject.
//
// Before the trim-on-output fix, the empty/null branch returned the
// canonical "{}" while the validated-object branch returned the
// caller's original bytes verbatim — so a payload like
// "  {\"a\":1}\n" round-tripped to disk with the surrounding
// whitespace intact, producing byte-different rows for callers that
// happened to format their input differently. This test would catch
// any regression that re-introduces the inconsistency by asserting
// that the returned slice has no boundary whitespace and that the
// top-level JSON token is preserved exactly.
func TestNormalizeJSONObject_trimsLeadingAndTrailingWhitespace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `{"a":1}`, `{"a":1}`},
		{"leading spaces", `   {"a":1}`, `{"a":1}`},
		{"trailing newline", `{"a":1}` + "\n", `{"a":1}`},
		{"mixed boundary whitespace", "\t  {\"a\":1}  \n", `{"a":1}`},
		{"interior whitespace preserved", "{ \"a\": 1 , \"b\": 2 }", "{ \"a\": 1 , \"b\": 2 }"},
		{"empty string", "", "{}"},
		{"all whitespace", "   \n\t  ", "{}"},
		{"json null", "null", "{}"},
		{"padded json null", "  null  ", "{}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeJSONObject([]byte(tc.in), "field")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("NormalizeJSONObject(%q) = %q, want %q (boundary whitespace must be stripped so on-disk bytes are canonical regardless of which branch fires)", tc.in, got, tc.want)
			}
			if len(got) > 0 {
				if got[0] == ' ' || got[0] == '\t' || got[0] == '\n' || got[0] == '\r' {
					t.Errorf("returned bytes start with whitespace (%q) — boundary trim must apply to the validated-object branch as well as the canonical-empty branch", got)
				}
				last := got[len(got)-1]
				if last == ' ' || last == '\t' || last == '\n' || last == '\r' {
					t.Errorf("returned bytes end with whitespace (%q) — boundary trim must apply to the validated-object branch as well as the canonical-empty branch", got)
				}
			}
			var probe any
			if err := json.Unmarshal(got, &probe); err != nil {
				t.Fatalf("returned bytes must remain valid JSON: %v (raw=%q)", err, got)
			}
			if _, ok := probe.(map[string]any); !ok {
				t.Errorf("returned bytes must decode to a JSON object; got %T", probe)
			}
		})
	}
}

// TestNormalizeJSONObject_rejectsNonObjects guards the existing
// type-checking contract so the trim-on-output change does not weaken
// the rejection of non-object payloads.
func TestNormalizeJSONObject_rejectsNonObjects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{"number", "42"},
		{"string", `"hello"`},
		{"array", `[1,2,3]`},
		{"bool", "true"},
		{"malformed", `{"a":`},
		{"trailing garbage", `{"a":1} junk`},
		{"padded number", "  42  "},
		{"padded array", "  [1,2,3]  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeJSONObject([]byte(tc.in), "field")
			if err == nil {
				t.Fatalf("NormalizeJSONObject(%q) must reject non-object input", tc.in)
			}
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("error must wrap domain.ErrInvalidInput so handlers map to 400; got %v", err)
			}
			if !strings.Contains(err.Error(), "field") {
				t.Errorf("error must include the field name to aid debugging; got %v", err)
			}
		})
	}
}

// TestNormalizeJSONObject_idempotent asserts that re-normalizing the
// output of a prior NormalizeJSONObject call is a fixed point. Without
// this property the dual-write audit mirror could observe drift if a
// payload travelled through the kernel more than once.
func TestNormalizeJSONObject_idempotent(t *testing.T) {
	t.Parallel()

	inputs := [][]byte{
		[]byte(`  {"x":1}  `),
		[]byte("\n{\"x\":1}\n"),
		[]byte("null"),
		[]byte(""),
		[]byte("   "),
	}
	for _, in := range inputs {
		first, err := NormalizeJSONObject(in, "field")
		if err != nil {
			t.Fatalf("first pass failed for %q: %v", in, err)
		}
		second, err := NormalizeJSONObject(first, "field")
		if err != nil {
			t.Fatalf("second pass failed for %q (first=%q): %v", in, first, err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("normalize is not idempotent for %q: first=%q second=%q", in, first, second)
		}
	}
}
