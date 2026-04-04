package envload

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func findRepoRoot(startDir string) (string, error) {
	slog.Debug("trace", "operation", "envload.findRepoRoot")
	dir := startDir
	for {
		mod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(mod); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", mod, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found (run from inside this repository)")
		}
		dir = parent
	}
}

func resolveDotenvPath(workingDir, flagPath string) (string, error) {
	slog.Debug("trace", "operation", "envload.resolveDotenvPath")
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
	slog.Debug("trace", "operation", "envload.Load")
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	path, err = resolveDotenvPath(wd, envFileOverride)
	if err != nil {
		return "", fmt.Errorf("resolve .env path: %w", err)
	}
	if err := godotenv.Overload(path); err != nil {
		return path, fmt.Errorf("godotenv overload %q: %w", path, err)
	}
	if os.Getenv("DATABASE_URL") == "" {
		return path, fmt.Errorf("DATABASE_URL is empty after loading %s", path)
	}
	return path, nil
}

// OverloadDotenvIfPresent runs godotenv.Overload on the resolved .env path when the file exists.
// It does not require DATABASE_URL. If the file is missing, it succeeds without changing the environment.
// Callers use this before full Load when variables from .env must be visible early (for example taskapi logging flags).
func OverloadDotenvIfPresent(envFileOverride string) (resolvedPath string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	path, err := resolveDotenvPath(wd, envFileOverride)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return path, nil
		}
		return path, statErr
	}
	if err := godotenv.Overload(path); err != nil {
		return path, fmt.Errorf("godotenv overload %q: %w", path, err)
	}
	return path, nil
}
