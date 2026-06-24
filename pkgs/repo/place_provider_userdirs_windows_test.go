//go:build windows

package repo

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// TestUserDirsProvider_includesRedirectedDocumentsOnWindows verifies that
// UserDirsPlaceProvider resolves the OneDrive-redirected Documents path via
// KnownFolderPath rather than $HOME/Documents. The provider is no longer in
// defaultPlaceRegistry (retired in Cycle 7) but its resolution logic is kept
// for potential reuse.
func TestUserDirsProvider_includesRedirectedDocumentsOnWindows(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	documentsKnown, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		t.Fatal(err)
	}
	defaultDocuments := filepath.Join(home, "Documents")
	if filepath.Clean(documentsKnown) == filepath.Clean(defaultDocuments) {
		t.Skip("Documents is not redirected on this host")
	}

	places, err := UserDirsPlaceProvider{}.Places(BrowseEnvNative, home)
	if err != nil {
		t.Fatal(err)
	}
	var documentsPlace *Place
	for i := range places {
		if places[i].Category == PlaceCategoryDocuments {
			documentsPlace = &places[i]
			break
		}
	}
	if documentsPlace == nil {
		t.Fatal("expected Documents place from UserDirsPlaceProvider")
	}
	if filepath.Clean(documentsPlace.Path) != filepath.Clean(documentsKnown) {
		t.Fatalf("Documents path = %q want known folder %q", documentsPlace.Path, documentsKnown)
	}

	root := placeToBrowseRoot(*documentsPlace)
	listing, err := ListBrowseDirs([]BrowseRoot{root}, documentsPlace.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) < 10 {
		t.Fatalf("expected many folders under redirected Documents, got %d", len(listing.Entries))
	}
}

func TestResolveBrowseRoots_customEnvSkipsUserDirs(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("HAMIX_BROWSE_ROOTS", custom)
	roots, _, err := ResolveBrowseRoots(custom)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roots {
		if r.Category == PlaceCategoryDocuments {
			t.Fatalf("custom override should not include user dirs, got %+v", r)
		}
	}
}
