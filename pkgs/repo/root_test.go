package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if r.Abs() != dir {
		t.Fatalf("Abs = %q want %q", r.Abs(), dir)
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

func TestRoot_Ready_ok(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Ready(); err != nil {
		t.Fatal(err)
	}
}

func TestRoot_Ready_fails_when_directory_removed(t *testing.T) {
	dir := t.TempDir()
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := r.Ready(); err == nil {
		t.Fatal("expected error when root path is gone")
	}
}

func TestRoot_Search(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustMk := func(rel string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustMk("src/app.go")
	mustMk("lib/extra_foo.txt")
	mustMk(".git/HEAD")
	mustMk("node_modules/pkg/index.js")
	mustMk("vendor/v/mod.go")

	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty query lists tracked files skips special dirs", func(t *testing.T) {
		paths, err := r.Search("")
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			for _, prefix := range []string{".git/", "node_modules/", "vendor/"} {
				if strings.HasPrefix(p, prefix) || strings.Contains(p, "/"+prefix) {
					t.Fatalf("unexpected path under skipped dir: %q in %v", p, paths)
				}
			}
		}
		got := make(map[string]bool)
		for _, p := range paths {
			got[p] = true
		}
		for _, want := range []string{"src/app.go", "lib/extra_foo.txt"} {
			if !got[want] {
				t.Fatalf("missing %q in %v", want, paths)
			}
		}
	})

	t.Run("substring case insensitive", func(t *testing.T) {
		paths, err := r.Search("FOO")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 1 || paths[0] != "lib/extra_foo.txt" {
			t.Fatalf("paths = %v want [lib/extra_foo.txt]", paths)
		}
	})

	t.Run("no match", func(t *testing.T) {
		paths, err := r.Search("zzznonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Fatalf("paths = %v want []", paths)
		}
	})
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

func TestValidateRange_invalidBounds_checked_before_file_size(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "huge.txt")
	huge := strings.Repeat("a", maxFileReadBytes+1)
	if err := os.WriteFile(p, []byte(huge), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateRange(p, 0, 1)
	if err == nil {
		t.Fatal("expected invalid input error")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if !strings.Contains(err.Error(), "line numbers must be >= 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLineCount_rejects_files_larger_than_limit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	big := strings.Repeat("a", maxFileReadBytes+1)
	if err := os.WriteFile(p, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LineCount(p)
	if err == nil {
		t.Fatal("expected file too large error")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
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
