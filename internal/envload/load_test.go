package envload

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_explicitPath_loadsDATABASE_URL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.env")
	if err := os.WriteFile(p, []byte("DATABASE_URL=postgres://localhost/envload_test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "")
	path, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Clean(p) {
		t.Fatalf("path %q want %q", path, filepath.Clean(p))
	}
	if got := os.Getenv("DATABASE_URL"); got != "postgres://localhost/envload_test" {
		t.Fatalf("DATABASE_URL %q", got)
	}
}

func TestLoad_explicitPath_missingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.env")
	t.Setenv("DATABASE_URL", "")
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_emptyDATABASE_URL_afterFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "emptyurl.env")
	if err := os.WriteFile(p, []byte("OTHER=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "")
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error when DATABASE_URL unset after load")
	}
}

func TestOverloadDotenvIfPresent_loadsWithoutDATABASE_URL(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module envload_early_test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("T2A_DISABLE_LOGGING=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Setenv("T2A_DISABLE_LOGGING", "")
	p, err := OverloadDotenvIfPresent("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, ".env"); p != want {
		t.Fatalf("path %q want %q", p, want)
	}
	if os.Getenv("T2A_DISABLE_LOGGING") != "1" {
		t.Fatalf("expected T2A_DISABLE_LOGGING from .env")
	}
}

func TestOverloadDotenvIfPresent_missingFile_ok(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module envload_early_missing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	_, err = OverloadDotenvIfPresent("")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_resolveFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module envload_root_test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("DATABASE_URL=postgres://localhost/repo_root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "")
	path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".env")
	if path != want {
		t.Fatalf("path %q want %q", path, want)
	}
	if got := os.Getenv("DATABASE_URL"); got != "postgres://localhost/repo_root" {
		t.Fatalf("DATABASE_URL %q", got)
	}
}
