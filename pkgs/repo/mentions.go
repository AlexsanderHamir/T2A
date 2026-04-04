package repo

import (
	"log/slog"
	"strconv"
	"strings"
)

// Mention is one @path or @path(start-end) token in a prompt (1-based line range, inclusive).
type Mention struct {
	Path      string
	StartLine int
	EndLine   int
	HasRange  bool
	RawStart  int
	RawEnd    int
}

// ParseFileMentions extracts @-mentions. Paths may not contain whitespace; range uses (start-end).
func ParseFileMentions(s string) []Mention {
	slog.Debug("trace", "operation", "repo.ParseFileMentions")
	var out []Mention
	i := 0
outer:
	for i < len(s) {
		j := strings.Index(s[i:], "@")
		if j < 0 {
			break
		}
		i += j
		rawStart := i
		i++
		pathStart := i
		for i < len(s) {
			c := s[i]
			if c == '(' {
				path := strings.TrimSpace(s[pathStart:i])
				if path == "" {
					i = rawStart + 1
					continue outer
				}
				closeIdx := strings.IndexByte(s[i:], ')')
				if closeIdx < 0 {
					i++
					break
				}
				inner := strings.TrimSpace(s[i+1 : i+closeIdx])
				dash := strings.IndexByte(inner, '-')
				var startLine, endLine int
				hasRange := true
				if dash >= 0 {
					a, err1 := strconv.Atoi(strings.TrimSpace(inner[:dash]))
					b, err2 := strconv.Atoi(strings.TrimSpace(inner[dash+1:]))
					if err1 == nil && err2 == nil {
						startLine, endLine = a, b
					}
				}
				i += closeIdx + 1
				out = append(out, Mention{
					Path:      path,
					StartLine: startLine,
					EndLine:   endLine,
					HasRange:  hasRange,
					RawStart:  rawStart,
					RawEnd:    i,
				})
				continue outer
			}
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '@' {
				break
			}
			i++
		}
		path := strings.TrimSpace(s[pathStart:i])
		if path != "" {
			out = append(out, Mention{
				Path:     path,
				RawStart: rawStart,
				RawEnd:   i,
			})
		}
	}
	return out
}
