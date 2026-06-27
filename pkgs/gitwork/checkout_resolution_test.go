package gitwork_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
)

func registeredFromMain(t *testing.T, main string) gitwork.RegisteredCheckout {
	t.Helper()
	repo := openRepoAt(t, main)
	branches, err := svc().ListBranches(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	heads := make(map[string]string, len(branches))
	for _, b := range branches {
		if b.Name != "" && b.HeadSHA != "" {
			heads[b.Name] = b.HeadSHA
		}
	}
	return gitwork.RegisteredCheckout{
		CachedMainPath:  main,
		CachedCommonDir: repo.CommonDir,
		BranchHeads:     heads,
	}
}

func openRepoAt(t *testing.T, path string) *gitwork.Repository {
	t.Helper()
	repo, err := svc().OpenRepository(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenRepository(%q): %v", path, err)
	}
	return repo
}

func TestVerifySameRepository_matchingBranchHead(t *testing.T) {
	main := initRepo(t)
	registered := registeredFromMain(t, main)
	candidate := openRepoAt(t, main)
	if err := svc().VerifySameRepository(context.Background(), registered, candidate); err != nil {
		t.Fatalf("VerifySameRepository: %v", err)
	}
}

func TestVerifySameRepository_matchingCommonDirNoBranches(t *testing.T) {
	main := initRepo(t)
	repo := openRepoAt(t, main)
	registered := gitwork.RegisteredCheckout{
		CachedMainPath:  main,
		CachedCommonDir: repo.CommonDir,
	}
	if err := svc().VerifySameRepository(context.Background(), registered, repo); err != nil {
		t.Fatalf("VerifySameRepository: %v", err)
	}
}

func TestVerifySameRepository_mismatch(t *testing.T) {
	mainA := initRepo(t)
	runGit(t, mainA, "commit", "--allow-empty", "-m", "unique-a")
	mainB := initRepo(t)
	registered := registeredFromMain(t, mainA)
	candidate := openRepoAt(t, mainB)
	err := svc().VerifySameRepository(context.Background(), registered, candidate)
	if !errors.Is(err, gitwork.ErrBootstrapMismatch) {
		t.Fatalf("got %v want ErrBootstrapMismatch", err)
	}
}

func TestOpenRegisteredCheckout_cacheHit(t *testing.T) {
	main := initRepo(t)
	registered := registeredFromMain(t, main)
	result, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered: registered,
	})
	if err != nil {
		t.Fatalf("OpenRegisteredCheckout: %v", err)
	}
	if result.Source != gitwork.ResolveSourceCache {
		t.Fatalf("source=%q want cache", result.Source)
	}
	if result.Repo == nil {
		t.Fatal("expected repo")
	}
}

func TestOpenRegisteredCheckout_candidateAfterRename(t *testing.T) {
	main := initRepo(t)
	registered := registeredFromMain(t, main)
	renamed := filepath.Join(filepath.Dir(main), "renamed-open")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, main) })

	result, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered:    registered,
		CandidatePath: renamed,
	})
	if err != nil {
		t.Fatalf("OpenRegisteredCheckout: %v", err)
	}
	if result.Source != gitwork.ResolveSourceCandidate {
		t.Fatalf("source=%q want candidate", result.Source)
	}
	if result.OpenedPath == "" {
		t.Fatal("expected opened path")
	}
}

func TestOpenRegisteredCheckout_candidateMismatch(t *testing.T) {
	mainA := initRepo(t)
	runGit(t, mainA, "commit", "--allow-empty", "-m", "a")
	mainB := initRepo(t)
	registered := registeredFromMain(t, mainA)
	renamed := filepath.Join(filepath.Dir(mainA), "gone-a")
	if err := os.Rename(mainA, renamed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, mainA) })

	_, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered:    registered,
		CandidatePath: mainB,
	})
	if !errors.Is(err, gitwork.ErrBootstrapMismatch) {
		t.Fatalf("got %v want ErrBootstrapMismatch", err)
	}
}

func TestOpenRegisteredCheckout_needsBootstrapWhenMissing(t *testing.T) {
	main := initRepo(t)
	registered := registeredFromMain(t, main)
	renamed := filepath.Join(filepath.Dir(main), "renamed-missing")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, main) })

	result, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered: registered,
	})
	if err != nil {
		t.Fatalf("OpenRegisteredCheckout: %v", err)
	}
	if result.Repo != nil {
		t.Fatal("expected nil repo when bootstrap needed")
	}
}

func TestDiscoverCheckoutNearby_singleMatch(t *testing.T) {
	main := initRepo(t)
	registered := registeredFromMain(t, main)
	renamed := filepath.Join(filepath.Dir(main), "discovered-main")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, main) })

	result, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered:    registered,
		AllowDiscover: true,
	})
	if err != nil {
		t.Fatalf("OpenRegisteredCheckout: %v", err)
	}
	if result.Source != gitwork.ResolveSourceDiscovered {
		t.Fatalf("source=%q want discovered", result.Source)
	}
}

func copyDirForTest(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copy dir %q -> %q: %v", src, dst, err)
	}
}

func TestDiscoverCheckoutNearby_ambiguous(t *testing.T) {
	mainA := initRepo(t)
	runGit(t, mainA, "commit", "--allow-empty", "-m", "marker")
	registered := registeredFromMain(t, mainA)
	parent := filepath.Dir(mainA)
	copyMain := filepath.Join(parent, "copy-main")
	copyDirForTest(t, mainA, copyMain)
	renamedMain := filepath.Join(parent, "moved-main")
	if err := os.Rename(mainA, renamedMain); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Rename(renamedMain, mainA)
		_ = os.RemoveAll(copyMain)
	})
	_, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered:    registered,
		AllowDiscover: true,
	})
	if !errors.Is(err, gitwork.ErrAmbiguousDiscovery) {
		t.Fatalf("got %v want ErrAmbiguousDiscovery", err)
	}
}

func TestDiscoverCheckoutNearby_none(t *testing.T) {
	main := initRepo(t)
	registered := registeredFromMain(t, main)
	elsewhere := filepath.Join(t.TempDir(), "moved-away")
	if err := os.Rename(main, elsewhere); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(elsewhere) })

	result, err := svc().OpenRegisteredCheckout(context.Background(), gitwork.ResolveInput{
		Registered:    registered,
		AllowDiscover: true,
	})
	if err != nil {
		t.Fatalf("OpenRegisteredCheckout: %v", err)
	}
	if result.Repo != nil {
		t.Fatal("expected no discovery when checkout left parent directory")
	}
}
