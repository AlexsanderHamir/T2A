package main

import (
	"errors"
	"os"
	"path/filepath"
)

func findRepoRoot(startDir string) (string, error) {
	dir := startDir
	for {
		mod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(mod); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found (run from inside this repository)")
		}
		dir = parent
	}
}

func resolveDotenvPath(workingDir, flagPath string) (string, error) {
	if flagPath != "" {
		return filepath.Clean(flagPath), nil
	}
	root, err := findRepoRoot(workingDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".env"), nil
}
