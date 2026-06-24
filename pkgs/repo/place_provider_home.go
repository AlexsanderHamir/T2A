package repo

// HomePlaceProvider returns the operator home directory as a picker root.
type HomePlaceProvider struct{}

// Places implements PlaceProvider.
func (HomePlaceProvider) Places(env BrowseEnvironment, _ string) ([]Place, error) {
	root, err := resolveHomeBrowseRoot(env)
	if err != nil {
		return nil, nil
	}
	return []Place{{
		ID:                root.ID,
		Path:              root.Path,
		Label:             root.Label,
		Category:          PlaceCategoryHome,
		Available:         root.Available,
		UnavailableReason: root.UnavailableReason,
	}}, nil
}
