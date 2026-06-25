package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// TestHTTP_workspaceRoots_returnsRegisteredRepos verifies that registered git
// repositories are returned as workspace roots with category "registered".
func TestHTTP_workspaceRoots_returnsRegisteredRepos(t *testing.T) {
	repoPath := t.TempDir()

	db := tasktestdb.OpenSQLite(t)
	now := time.Now().UTC()
	row := domain.GitRepository{
		ID:            "test-repo-id",
		Path:          repoPath,
		HostPath:      "",
		DefaultBranch: "main",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed repo: %v", err)
	}

	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/workspace-roots")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body workspaceRootsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Environment != "native" {
		t.Fatalf("environment=%q", body.Environment)
	}
	if len(body.Roots) != 1 || body.Roots[0].Path != repoPath {
		t.Fatalf("roots=%+v", body.Roots)
	}
	if body.Roots[0].Category != repo.PlaceCategoryRegistered {
		t.Fatalf("category=%q want registered", body.Roots[0].Category)
	}
}

// TestHTTP_workspaceRoots_bootstrapWhenNoRepos verifies OS bootstrap entry points
// when no repositories are registered and HAMIX_BROWSE_ROOTS is unset.
func TestHTTP_workspaceRoots_bootstrapWhenNoRepos(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/workspace-roots")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body workspaceRootsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Roots) == 0 {
		t.Fatal("expected bootstrap roots, got empty list")
	}
	var hasHome bool
	for _, root := range body.Roots {
		if root.Category == repo.PlaceCategoryHome && root.Available {
			hasHome = true
			break
		}
	}
	if !hasHome {
		t.Fatalf("expected available home bootstrap root, got %+v", body.Roots)
	}
}

// TestHTTP_workspaceRoots_customOverrideReplacesDB verifies that HAMIX_BROWSE_ROOTS
// replaces DB-sourced roots when set (ops override pattern).
func TestHTTP_workspaceRoots_customOverrideReplacesDB(t *testing.T) {
	customRoot := t.TempDir()
	t.Setenv("HAMIX_BROWSE_ROOTS", customRoot)

	db := tasktestdb.OpenSQLite(t)
	now := time.Now().UTC()
	row := domain.GitRepository{
		ID:            "ignored-repo",
		Path:          t.TempDir(),
		DefaultBranch: "main",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed repo: %v", err)
	}

	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/workspace-roots")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body workspaceRootsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Roots) != 1 || body.Roots[0].Path != customRoot {
		t.Fatalf("expected custom root %s, got %+v", customRoot, body.Roots)
	}
	if body.Roots[0].Category != repo.PlaceCategoryCustom {
		t.Fatalf("category=%q want custom", body.Roots[0].Category)
	}
}

func TestHTTP_browseDirs_listsProjectFolder(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "my-app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HAMIX_BROWSE_ROOTS", root)

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/browse-dirs?path=" + url.QueryEscape(root))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body browseDirsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Entries) != 1 || body.Entries[0].Name != "my-app" {
		t.Fatalf("entries=%+v", body.Entries)
	}
}

// TestHTTP_browseDirs_fullDiskFallback verifies that when HAMIX_BROWSE_ROOTS is unset,
// browseDirs performs full-disk (unrestricted) listing for the register-repo bootstrap flow.
func TestHTTP_browseDirs_fullDiskFallback(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "my-project")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/browse-dirs?path=" + url.QueryEscape(root))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body browseDirsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Entries) != 1 || body.Entries[0].Name != "my-project" {
		t.Fatalf("entries=%+v", body.Entries)
	}
}

func TestHTTP_browseDirs_worksWithoutRepoRootConfigured(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HAMIX_BROWSE_ROOTS", root)

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/browse-dirs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
}
