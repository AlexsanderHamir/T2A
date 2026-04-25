package adapterkit

import (
	"regexp"
	"strings"
)

const RedactedValue = "[REDACTED]"

var (
	authHeaderRE   = regexp.MustCompile(`(?i)(authorization:[ \t]*)([^\r\n]+)`)
	cookieHeaderRE = regexp.MustCompile(`(?i)\b((?:set-)?cookie:[ \t]*)([^\r\n]+)`)
)

// RedactionPolicy controls the baseline redaction applied before runner output
// is persisted.
type RedactionPolicy struct {
	HomePaths   []string
	EnvPrefixes []string
}

// DefaultRedactionPolicy returns the repository's shared redaction floor.
func DefaultRedactionPolicy(homePaths []string) RedactionPolicy {
	return RedactionPolicy{
		HomePaths:   append([]string(nil), homePaths...),
		EnvPrefixes: []string{"T2A_"},
	}
}

// Redact replaces secret-shaped substrings in s and rewrites configured home
// paths to "~".
func Redact(s string, policy RedactionPolicy) string {
	if s == "" {
		return s
	}
	out := authHeaderRE.ReplaceAllString(s, "${1}"+RedactedValue)
	out = cookieHeaderRE.ReplaceAllString(out, "${1}"+RedactedValue)
	for _, prefix := range policy.EnvPrefixes {
		if prefix == "" {
			continue
		}
		out = redactEnvAssignments(out, prefix)
	}
	for _, hp := range policy.HomePaths {
		if hp == "" {
			continue
		}
		out = strings.ReplaceAll(out, hp, "~")
	}
	return out
}

func redactEnvAssignments(s, prefix string) string {
	pattern := regexp.MustCompile(`(` + regexp.QuoteMeta(prefix) + `[A-Z0-9_]+)\s*=\s*\S+`)
	return pattern.ReplaceAllString(s, "$1="+RedactedValue)
}
