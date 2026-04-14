package repo

import (
	"log/slog"
	"strconv"
	"strings"
)

func parseMentionLineRangeInner(inner string) (startLine, endLine int, ok bool) {
	slog.Debug("trace", "operation", "repo.parseMentionLineRangeInner")
	dash := strings.IndexByte(inner, '-')
	if dash < 0 {
		return 0, 0, false
	}
	a, err1 := strconv.Atoi(strings.TrimSpace(inner[:dash]))
	b, err2 := strconv.Atoi(strings.TrimSpace(inner[dash+1:]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return a, b, true
}

// handleMentionOpenParen runs when s[i] == '(' inside a @path token.
// If restartOuter is true, set i to restartFrom and continue the outer loop.
// If continueOuter is true, mention is non-nil and the caller should append it and continue outer.
// Otherwise newI is the next index inside the inner path scan (typically i+1 after a failed parse).
func handleMentionOpenParen(s string, i, pathStart, rawStart int) (
	newI int,
	mention *Mention,
	continueOuter bool,
	restartFrom int,
	restartOuter bool,
) {
	slog.Debug("trace", "operation", "repo.handleMentionOpenParen")
	closeRel := strings.IndexByte(s[i:], ')')
	if closeRel < 0 {
		return i + 1, nil, false, 0, false
	}
	inner := strings.TrimSpace(s[i+1 : i+closeRel])
	startLine, endLine, hasRange := parseMentionLineRangeInner(inner)
	if !hasRange {
		return i + 1, nil, false, 0, false
	}
	afterClose := i + closeRel + 1
	if afterClose < len(s) && !isMentionDelimiter(s[afterClose]) {
		return i + 1, nil, false, 0, false
	}
	path := strings.TrimSpace(s[pathStart:i])
	if path == "" {
		return 0, nil, false, rawStart + 1, true
	}
	newI = i + closeRel + 1
	return newI, &Mention{
		Path:      path,
		StartLine: startLine,
		EndLine:   endLine,
		HasRange:  true,
		RawStart:  rawStart,
		RawEnd:    newI,
	}, true, 0, false
}
