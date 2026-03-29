package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestOpenRoot_and_Resolve(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg", "x.go")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := r.Resolve("pkg/x.go")
	if err != nil {
		t.Fatal(err)
	}
	if abs != sub {
		t.Fatalf("Resolve = %q want %q", abs, sub)
	}
	_, err = r.Resolve("../outside")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestLineCount_and_ValidateRange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := LineCount(p)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("LineCount = %d", n)
	}
	if err := ValidateRange(p, 1, 3); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRange(p, 2, 4); err == nil {
		t.Fatal("expected error for past EOF")
	}
}

func TestValidatePromptMentions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("l1\nl2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ValidatePromptMentions(`ref @ok.go(1-2)`); err != nil {
		t.Fatal(err)
	}
	err = r.ValidatePromptMentions(`ref @ok.go(1-9)`)
	if err == nil {
		t.Fatal("expected error for bad range")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
