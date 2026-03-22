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

func TestLoad_resolveFromRepoRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("uses Chdir")
	}
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
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(old)
	}()
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
