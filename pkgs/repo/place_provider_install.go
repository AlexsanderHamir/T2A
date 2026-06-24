package repo

// InstallPlaceProvider returns the Hamix checkout directory as a picker root.
type InstallPlaceProvider struct{}

// Places implements PlaceProvider.
func (InstallPlaceProvider) Places(env BrowseEnvironment, startDir string) ([]Place, error) {
	root, err := resolveInstallBrowseRoot(startDir, env)
	if err != nil {
		return nil, nil
	}
	return []Place{{
		ID:                root.ID,
		Path:              root.Path,
		Label:             root.Label,
		Category:          PlaceCategoryInstall,
		Available:         root.Available,
		UnavailableReason: root.UnavailableReason,
	}}, nil
}
