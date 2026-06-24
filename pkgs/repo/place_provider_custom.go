package repo

import (
	"fmt"
	"os"
	"strings"
)

// CustomPlaceProvider returns roots from HAMIX_BROWSE_ROOTS when set.
type CustomPlaceProvider struct{}

// Places implements PlaceProvider.
func (CustomPlaceProvider) Places(_ BrowseEnvironment, _ string) ([]Place, error) {
	override := strings.TrimSpace(os.Getenv("HAMIX_BROWSE_ROOTS"))
	if override == "" {
		return nil, fmt.Errorf("HAMIX_BROWSE_ROOTS is not set")
	}
	roots, err := parseBrowseRootPaths(override)
	if err != nil {
		return nil, err
	}
	out := make([]Place, 0, len(roots))
	for _, r := range roots {
		out = append(out, Place{
			ID:                r.ID,
			Path:              r.Path,
			Label:             r.Label,
			Category:          PlaceCategoryCustom,
			Available:         r.Available,
			UnavailableReason: r.UnavailableReason,
		})
	}
	return out, nil
}

// CustomBrowseRootsConfigured reports whether HAMIX_BROWSE_ROOTS replaces defaults.
func CustomBrowseRootsConfigured() bool {
	return strings.TrimSpace(os.Getenv("HAMIX_BROWSE_ROOTS")) != ""
}
