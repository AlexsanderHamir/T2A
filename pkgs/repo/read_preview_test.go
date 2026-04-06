package repo

import (
	"strings"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFilePreview_text(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := ReadFilePreview(p)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Binary {
		t.Fatal("expected text")
	}
	if fp.Content != "a\nb\nc" {
		t.Fatalf("content %q", fp.Content)
	}
	if fp.LineCount != 3 {
		t.Fatalf("line_count %d", fp.LineCount)
	}
}

func TestReadFilePreview_binaryNUL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(p, []byte("hello\x00world"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := ReadFilePreview(p)
	if err != nil {
		t.Fatal(err)
	}
	if !fp.Binary {
		t.Fatal("expected binary")
	}
	if fp.Content != "" {
		t.Fatal("expected empty content")
	}
}

func TestReadFilePreview_truncatedLargeText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "large.txt")
	content := strings.Repeat("a\n", (maxFileReadBytes/2)+100)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := ReadFilePreview(p)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Binary {
		t.Fatal("expected text")
	}
	if !fp.Truncated {
		t.Fatal("expected truncated preview")
	}
	if len(fp.Content) != maxFileReadBytes {
		t.Fatalf("content length %d", len(fp.Content))
	}
	if fp.LineCount <= 0 {
		t.Fatalf("line_count %d", fp.LineCount)
	}
}
