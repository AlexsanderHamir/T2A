//go:build linux

package repo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var xdgUserDirKeys = []struct {
	key      string
	category PlaceCategory
	label    string
	id       string
}{
	{"XDG_DESKTOP_DIR", PlaceCategoryDesktop, "Desktop", "desktop"},
	{"XDG_DOCUMENTS_DIR", PlaceCategoryDocuments, "Documents", "documents"},
	{"XDG_DOWNLOAD_DIR", PlaceCategoryDownloads, "Downloads", "downloads"},
	{"XDG_PICTURES_DIR", PlaceCategoryPictures, "Pictures", "pictures"},
	{"XDG_MUSIC_DIR", PlaceCategoryMusic, "Music", "music"},
	{"XDG_VIDEOS_DIR", PlaceCategoryVideos, "Videos", "videos"},
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func resolveUserDirPlaces() ([]Place, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return parseXDGUserDirsFile(filepath.Join(configHome, "user-dirs.dirs"), home)
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func parseXDGUserDirsFile(path, home string) ([]Place, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultLinuxUserDirPlaces(home)
		}
		return nil, err
	}
	defer f.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"`)
		values[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	out := make([]Place, 0, len(xdgUserDirKeys))
	for _, spec := range xdgUserDirKeys {
		raw, ok := values[spec.key]
		if !ok {
			continue
		}
		path, err := expandXDGPath(raw, home)
		if err != nil {
			continue
		}
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, Place{
			ID:        spec.id,
			Path:      filepath.Clean(path),
			Label:     spec.label,
			Category:  spec.category,
			Available: true,
		})
	}
	if len(out) == 0 {
		return defaultLinuxUserDirPlaces(home)
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Browse sub-step; operation trace is emitted by ResolveBrowseRoots."
func defaultLinuxUserDirPlaces(home string) ([]Place, error) {
	specs := []struct {
		category PlaceCategory
		dir      string
		label    string
		id       string
	}{
		{PlaceCategoryDesktop, "Desktop", "Desktop", "desktop"},
		{PlaceCategoryDocuments, "Documents", "Documents", "documents"},
		{PlaceCategoryDownloads, "Downloads", "Downloads", "downloads"},
		{PlaceCategoryPictures, "Pictures", "Pictures", "pictures"},
		{PlaceCategoryMusic, "Music", "Music", "music"},
		{PlaceCategoryVideos, "Videos", "Videos", "videos"},
	}
	out := make([]Place, 0, len(specs))
	for _, spec := range specs {
		path := filepath.Join(home, spec.dir)
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, Place{
			ID:        spec.id,
			Path:      filepath.Clean(path),
			Label:     spec.label,
			Category:  spec.category,
			Available: true,
		})
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ResolveBrowseRoots."
func expandXDGPath(raw, home string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty xdg path")
	}
	if strings.HasPrefix(raw, "$HOME/") {
		return filepath.Join(home, strings.TrimPrefix(raw, "$HOME/")), nil
	}
	if raw == "$HOME" {
		return home, nil
	}
	if filepath.IsAbs(raw) {
		return raw, nil
	}
	return "", fmt.Errorf("unsupported xdg path %q", raw)
}
