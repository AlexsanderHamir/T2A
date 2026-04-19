package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
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

// TestReadFilePreview_truncatedDoesNotSplitMultibyteUTF8 pins the contract
// that the byte-cap truncation in readCappedUTF8Content must not mis-label
// a valid UTF-8 file as binary just because the byte boundary lands in the
// middle of a multi-byte rune. Before the fix, a file consisting entirely
// of valid 3-byte UTF-8 characters that does not happen to be 3-byte
// aligned at maxFileReadBytes had its trailing partial rune left in
// the buffer, `utf8.Valid` returned false in applyBytesToPreview, and the
// preview was returned as Binary=true with empty Content — even though the
// file is plain text. The fix backtracks past any trailing partial rune
// (UTFMax-1 bytes max) before the validity check.
//
// This test uses the 3-byte UTF-8 character "の" (U+306E) and writes
// (maxFileReadBytes/3)+1 of them, guaranteeing the cap lands inside the
// final rune's encoding.
func TestReadFilePreview_truncatedDoesNotSplitMultibyteUTF8(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "ja.txt")
	const ch = "の" // 3 bytes in UTF-8
	count := (maxFileReadBytes / len(ch)) + 1
	content := strings.Repeat(ch, count)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := ReadFilePreview(p)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Binary {
		t.Fatalf("expected text preview, got Binary=true (truncation split a UTF-8 rune)")
	}
	if !fp.Truncated {
		t.Fatalf("expected Truncated=true (file is larger than maxFileReadBytes)")
	}
	if fp.Content == "" {
		t.Fatalf("expected non-empty Content; got empty")
	}
	if !utf8.ValidString(fp.Content) {
		t.Fatalf("preview Content is not valid UTF-8")
	}
	if got := len(fp.Content); got > maxFileReadBytes {
		t.Fatalf("preview Content length %d exceeds maxFileReadBytes %d", got, maxFileReadBytes)
	}
	if got := len(fp.Content); got < maxFileReadBytes-utf8.UTFMax {
		t.Fatalf("preview Content length %d shrank by more than UTFMax bytes from cap %d", got, maxFileReadBytes)
	}
}

func TestReadFilePreview_binaryLargeFileFlagsTruncated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.bin")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Truncate(maxFileReadBytes + 10); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	fp, err := ReadFilePreview(p)
	if err != nil {
		t.Fatal(err)
	}
	if !fp.Binary {
		t.Fatal("expected binary")
	}
	if !fp.Truncated {
		t.Fatal("expected truncated")
	}
	if fp.Content != "" {
		t.Fatal("expected empty content")
	}
	if fp.SizeBytes != maxFileReadBytes+10 {
		t.Fatalf("size_bytes %d", fp.SizeBytes)
	}
}
