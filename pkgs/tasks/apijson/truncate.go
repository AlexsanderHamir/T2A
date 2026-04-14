package apijson

import (
	"context"
	"log/slog"
	"unicode/utf8"
)

const maxJSONLogPreviewBytes = 16384

func truncateUTF8ByBytes(s string, maxBytes int) string {
	_ = slog.Default().Enabled(context.Background(), slog.LevelDebug)
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
