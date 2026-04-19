package apijson

import (
	"unicode/utf8"
)

const maxJSONLogPreviewBytes = 16384

// TruncateUTF8ByBytes returns s truncated to maxBytes UTF-8-safe, appending
// "…" when truncated. Pure helper: callers (debugHTTPRequest /
// debugHTTPResponse) already gate on slog.Default().Enabled, so per-call
// logging here would just duplicate the surrounding trace line.
// Skip-listed in cmd/funclogmeasure/analyze.go.
func TruncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	const ell = "…"
	if maxBytes <= len(ell) {
		return ell[:maxBytes]
	}
	limit := maxBytes - len(ell)
	end := 0
	for i := 0; i < len(s); {
		_, sz := utf8.DecodeRuneInString(s[i:])
		if i+sz > limit {
			break
		}
		i += sz
		end = i
	}
	if end == 0 {
		return ell
	}
	return s[:end] + ell
}
