package repo

import (
	"fmt"
)

// PlaceCategory identifies a well-known workspace picker location.
type PlaceCategory string

const (
	PlaceCategoryInstall    PlaceCategory = "install"
	PlaceCategoryHome       PlaceCategory = "home"
	PlaceCategoryDocuments  PlaceCategory = "documents"
	PlaceCategoryDesktop    PlaceCategory = "desktop"
	PlaceCategoryDownloads  PlaceCategory = "downloads"
	PlaceCategoryPictures   PlaceCategory = "pictures"
	PlaceCategoryMusic      PlaceCategory = "music"
	PlaceCategoryVideos     PlaceCategory = "videos"
	PlaceCategoryCustom     PlaceCategory = "custom"
	PlaceCategoryRegistered PlaceCategory = "registered"
)

// Place is a labeled absolute directory the workspace picker may start from.
type Place struct {
	ID                string
	Path              string
	Label             string
	Category          PlaceCategory
	Available         bool
	UnavailableReason string
}

// PlaceProvider returns zero or more Places for the current process environment.
type PlaceProvider interface {
	Places(env BrowseEnvironment, startDir string) ([]Place, error)
}

// PlaceRegistry composes providers in registration order and dedupes by
// canonical path so the same directory is not listed twice.
type PlaceRegistry struct {
	providers []PlaceProvider
}

// NewPlaceRegistry builds a registry from one or more providers.
//
//funclogmeasure:skip category=hot-path reason="Pure constructor; operation trace is emitted by ResolveBrowseRoots."
func NewPlaceRegistry(providers ...PlaceProvider) *PlaceRegistry {
	return &PlaceRegistry{providers: providers}
}

// Places returns all unique Places from registered providers.
//
//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func (r *PlaceRegistry) Places(env BrowseEnvironment, startDir string) ([]Place, error) {
	if r == nil || len(r.providers) == 0 {
		return nil, fmt.Errorf("no place providers registered")
	}
	seen := make(map[string]struct{})
	out := make([]Place, 0, 8)
	for _, provider := range r.providers {
		places, err := provider.Places(env, startDir)
		if err != nil {
			return nil, err
		}
		for _, place := range places {
			canon, err := canonicalizePathForContainment(place.Path)
			if err != nil {
				continue
			}
			if _, dup := seen[canon]; dup {
				continue
			}
			seen[canon] = struct{}{}
			out = append(out, place)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no browse roots available")
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ResolveBrowseRoots."
func placeToBrowseRoot(p Place) BrowseRoot {
	return BrowseRoot{
		ID:                p.ID,
		Path:              p.Path,
		Label:             p.Label,
		Category:          p.Category,
		Available:         p.Available,
		UnavailableReason: p.UnavailableReason,
	}
}
