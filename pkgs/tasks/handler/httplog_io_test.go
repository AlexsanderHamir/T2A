package handler

import (
	"strings"
	"testing"
)

func TestTruncateUTF8ByBytes_preservesShortString(t *testing.T) {
	if got := truncateUTF8ByBytes("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateUTF8ByBytes_truncatesASCII(t *testing.T) {
	s := strings.Repeat("a", 100)
	got := truncateUTF8ByBytes(s, 20)
	if len(got) > 20 {
		t.Fatalf("len %d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("want ellipsis suffix, got %q", got)
	}
}

func TestTruncateUTF8ByBytes_truncatesMultibyte(t *testing.T) {
	s := "日本語テスト" + strings.Repeat("x", 200)
	got := truncateUTF8ByBytes(s, 24)
	if len(got) > 24 {
		t.Fatalf("len %d", len(got))
	}
}
