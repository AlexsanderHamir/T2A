package repo

import (
	"os"
	"path/filepath"
	"testing"
)

type staticPlaceProvider struct {
	places []Place
	err    error
}

func (p staticPlaceProvider) Places(_ BrowseEnvironment, _ string) ([]Place, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.places, nil
}

func TestPlaceRegistry_PreservesProviderOrder(t *testing.T) {
	t.Parallel()
	dirA := t.TempDir()
	dirB := t.TempDir()
	reg := NewPlaceRegistry(
		staticPlaceProvider{places: []Place{{ID: "a", Path: dirA, Label: "A", Category: PlaceCategoryHome, Available: true}}},
		staticPlaceProvider{places: []Place{{ID: "b", Path: dirB, Label: "B", Category: PlaceCategoryDocuments, Available: true}}},
	)
	got, err := reg.Places(BrowseEnvNative, dirA)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("got %+v", got)
	}
}

func TestPlaceRegistry_DedupesByCanonicalPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	link := filepath.Join(root, "linked")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported")
	}
	reg := NewPlaceRegistry(
		staticPlaceProvider{places: []Place{{ID: "first", Path: target, Label: "Target", Category: PlaceCategoryHome, Available: true}}},
		staticPlaceProvider{places: []Place{{ID: "second", Path: link, Label: "Link", Category: PlaceCategoryDocuments, Available: true}}},
	)
	got, err := reg.Places(BrowseEnvNative, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "first" {
		t.Fatalf("got %+v want first provider wins", got)
	}
}

func TestPlaceRegistry_NoProvidersRegistered(t *testing.T) {
	t.Parallel()
	reg := NewPlaceRegistry()
	if _, err := reg.Places(BrowseEnvNative, t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}

func TestPlaceRegistry_ProviderError(t *testing.T) {
	t.Parallel()
	reg := NewPlaceRegistry(staticPlaceProvider{err: os.ErrPermission})
	if _, err := reg.Places(BrowseEnvNative, t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}
