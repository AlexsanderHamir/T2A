package envload

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
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

func Load(envFileOverride string) (path string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	path, err = resolveDotenvPath(wd, envFileOverride)
	if err != nil {
		return "", err
	}
	if err := godotenv.Overload(path); err != nil {
		return path, fmt.Errorf("godotenv overload %q: %w", path, err)
	}
	if os.Getenv("DATABASE_URL") == "" {
		return path, fmt.Errorf("DATABASE_URL is empty after loading %s", path)
	}
	return path, nil
}
