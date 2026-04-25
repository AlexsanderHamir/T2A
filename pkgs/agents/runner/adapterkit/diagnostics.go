package adapterkit

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// CombineStreams returns one human-readable blob containing stdout and stderr,
// preserving which stream each section came from.
func CombineStreams(stdout, stderr []byte) string {
	var b strings.Builder
	b.Grow(len(stdout) + len(stderr) + 16)
	if len(stdout) > 0 {
		b.WriteString("[stdout]\n")
		b.Write(stdout)
		if !bytes.HasSuffix(stdout, []byte{'\n'}) {
			b.WriteByte('\n')
		}
	}
	if len(stderr) > 0 {
		b.WriteString("[stderr]\n")
		b.Write(stderr)
	}
	return b.String()
}

// ClipRunes clips s to maxRunes and appends an ellipsis when clipping occurs.
func ClipRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxRunes {
			b.WriteRune('…')
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// RedactedTail returns a redacted, UTF-8-safe trailing slice of b.
func RedactedTail(b []byte, policy RedactionPolicy, maxBytes int) string {
	if len(b) == 0 || maxBytes <= 0 {
		return ""
	}
	tail := b
	if len(tail) > maxBytes {
		tail = TrimLeadingPartialRune(tail[len(tail)-maxBytes:])
	}
	return Redact(string(tail), policy)
}

// TrimLeadingPartialRune drops leading UTF-8 continuation bytes from a byte
// slice cut that may have landed in the middle of a multibyte rune.
func TrimLeadingPartialRune(b []byte) []byte {
	for len(b) > 0 && !utf8.RuneStart(b[0]) {
		b = b[1:]
	}
	return b
}
