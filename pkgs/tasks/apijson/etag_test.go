package apijson

import (
	"net/http/httptest"
	"testing"
)

func TestComputeETag_is_stable_and_quoted(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	got := ComputeETag(body)
	if got == "" {
		t.Fatal("empty ETag")
	}
	if got[0] != '"' || got[len(got)-1] != '"' {
		t.Errorf("ETag must be quoted, got %q", got)
	}
	again := ComputeETag(body)
	if got != again {
		t.Errorf("ETag not stable: %q vs %q", got, again)
	}
}

func TestComputeETag_changes_with_body(t *testing.T) {
	a := ComputeETag([]byte(`{"v":1}`))
	b := ComputeETag([]byte(`{"v":2}`))
	if a == b {
		t.Errorf("different bodies produced same ETag: %q", a)
	}
}

func TestIfNoneMatchMatches(t *testing.T) {
	etag := `"abc123"`
	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{`"abc123"`, true},
		{`"other","abc123"`, true},
		{`"other", "abc123"`, true},
		{`W/"abc123"`, true},
		{`*`, true},
		{`"nope"`, false},
	}
	for _, c := range cases {
		if got := IfNoneMatchMatches(c.header, etag); got != c.want {
			t.Errorf("IfNoneMatchMatches(%q, %q) = %v, want %v", c.header, etag, got, c.want)
		}
	}
}

func TestApplyRevalidatableHeaders_sets_expected_directives(t *testing.T) {
	rec := httptest.NewRecorder()
	ApplyRevalidatableHeaders(rec)
	if got := rec.Header().Get("Cache-Control"); got != RevalidatableCacheControl {
		t.Errorf("Cache-Control = %q, want %q", got, RevalidatableCacheControl)
	}
	wantHeaders := map[string]string{
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=(), payment=()",
	}
	for k, v := range wantHeaders {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}
