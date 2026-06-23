package handler

import (
	"encoding/json"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

// PathMap translates container filesystem paths to host paths for JSON
// responses. Keys in HAMIX_PATH_MAP are container prefixes; values are the
// matching host prefixes. Inbound API paths remain container paths.
type PathMap struct {
	pairs []pathMapPair
}

type pathMapPair struct {
	container string
	host      string
}

// NewPathMapFromEnv loads HAMIX_PATH_MAP (JSON object). Invalid JSON logs a
// warning and yields an empty map so taskapi still starts.
func NewPathMapFromEnv() *PathMap {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewPathMapFromEnv")
	raw := strings.TrimSpace(os.Getenv("HAMIX_PATH_MAP"))
	if raw == "" {
		return &PathMap{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		slog.Warn("invalid HAMIX_PATH_MAP, ignoring", "cmd", calltrace.LogCmd, "operation", "handler.NewPathMapFromEnv", "err", err)
		return &PathMap{}
	}
	pairs := make([]pathMapPair, 0, len(m))
	for container, host := range m {
		container = strings.TrimSpace(container)
		host = strings.TrimSpace(host)
		if container == "" || host == "" {
			continue
		}
		pairs = append(pairs, pathMapPair{container: container, host: host})
	}
	sortPathMapPairs(pairs)
	return &PathMap{pairs: pairs}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func sortPathMapPairs(pairs []pathMapPair) {
	sort.Slice(pairs, func(i, j int) bool {
		if len(pairs[i].container) != len(pairs[j].container) {
			return len(pairs[i].container) > len(pairs[j].container)
		}
		return pairs[i].container < pairs[j].container
	})
}

// TranslateToHost maps a container path to its host equivalent using the
// longest matching container prefix. ok is false when no prefix matched.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (pm *PathMap) TranslateToHost(containerPath string) (host string, ok bool) {
	if pm == nil || len(pm.pairs) == 0 {
		return containerPath, false
	}
	for _, p := range pm.pairs {
		if !pathMapPrefixMatch(containerPath, p.container) {
			continue
		}
		suffix := strings.TrimPrefix(containerPath, p.container)
		return p.host + suffix, true
	}
	return containerPath, false
}

// DisplayHostPath returns the host-facing path for SPA display. When no map
// entry matches, the container path is returned unchanged.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (pm *PathMap) DisplayHostPath(containerPath string) string {
	if host, ok := pm.TranslateToHost(containerPath); ok {
		return host
	}
	return containerPath
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func pathMapPrefixMatch(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if prefix == "/" {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// WithPathMap wires container→host translation for JSON serializers.
func WithPathMap(pm *PathMap) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithPathMap")
	return func(h *Handler) {
		if pm != nil {
			h.pathMap = pm
		}
	}
}

// WithGitAvailable overrides the git binary probe (tests only).
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only HandlerOption wiring; no production operation boundary."
func WithGitAvailable(ok bool) HandlerOption {
	return func(h *Handler) {
		h.gitAvailable = ok
	}
}
