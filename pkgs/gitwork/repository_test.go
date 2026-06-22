package gitwork_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "README.md"), "init\n")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	all := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", all...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func svc() gitwork.Service {
	return gitwork.New()
}

func TestOpenRepository_happyPath(t *testing.T) {
	dir := initRepo(t)
	repo, err := svc().OpenRepository(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenRepository: %v", err)
	}
	wantRoot, _ := filepath.Abs(dir)
	wantRoot = filepath.ToSlash(wantRoot)
	if repo.Root != wantRoot {
		t.Fatalf("Root=%q want %q", repo.Root, wantRoot)
	}
	if repo.CommonDir == "" {
		t.Fatal("CommonDir empty")
	}
}

func TestOpenRepository_notARepository(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	_, err := svc().OpenRepository(context.Background(), dir)
	if err != gitwork.ErrNotARepository {
		t.Fatalf("got %v want ErrNotARepository", err)
	}
}
