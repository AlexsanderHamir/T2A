package repo

import (
	"log/slog"
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
				newI, mention, contOuter, restartFrom, restartOuter := handleMentionOpenParen(s, i, pathStart, rawStart)
				if restartOuter {
					i = restartFrom
					continue outer
				}
				i = newI
				if contOuter && mention != nil {
					out = append(out, *mention)
					continue outer
				}
				continue
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

func isMentionDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '@'
}
