package main

import (
	"fmt"
	"go/ast"
	"strings"
)

const (
	directivePrefix     = "//funclogmeasure:skip"
	minDirectiveReason  = 20
	categoryToolNoop    = "tool-required-noop"
	categoryHotPath     = "hot-path"
	categoryDelegate    = "delegate-already-logs"
	categoryReExport    = "re-export-wrapper"
)

var validSkipCategories = map[string]struct{}{
	categoryToolNoop: {},
	categoryHotPath:  {},
	categoryDelegate: {},
	categoryReExport: {},
}

type skipDirective struct {
	Category string
	Reason   string
}

func parseSkipDirective(fd *ast.FuncDecl) (*skipDirective, error) {
	if fd == nil || fd.Doc == nil {
		return nil, nil
	}
	for _, c := range fd.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if !strings.HasPrefix(text, "funclogmeasure:skip") {
			continue
		}
		return parseDirectiveFields(text)
	}
	return nil, nil
}

func parseDirectiveFields(text string) (*skipDirective, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(text, "funclogmeasure:skip"))
	if rest == "" {
		return nil, fmt.Errorf("empty funclogmeasure:skip directive")
	}
	fields := parseDirectiveKV(rest)
	cat := fields["category"]
	reason := fields["reason"]
	if cat == "" {
		return nil, fmt.Errorf("funclogmeasure:skip missing category")
	}
	if _, ok := validSkipCategories[cat]; !ok {
		return nil, fmt.Errorf("funclogmeasure:skip unknown category %q", cat)
	}
	if len(strings.TrimSpace(reason)) < minDirectiveReason {
		return nil, fmt.Errorf("funclogmeasure:skip reason must be at least %d characters", minDirectiveReason)
	}
	return &skipDirective{Category: cat, Reason: reason}, nil
}

func parseDirectiveKV(rest string) map[string]string {
	out := make(map[string]string)
	for rest != "" {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		eq := strings.Index(rest, "=")
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(rest[:eq])
		rest = strings.TrimSpace(rest[eq+1:])
		if rest == "" {
			break
		}
		if rest[0] == '"' {
			end := strings.Index(rest[1:], "\"")
			if end < 0 {
				out[key] = strings.Trim(rest, "\"")
				break
			}
			out[key] = rest[1 : 1+end]
			rest = strings.TrimSpace(rest[2+end:])
			continue
		}
		sp := strings.IndexAny(rest, " \t")
		if sp < 0 {
			out[key] = rest
			break
		}
		out[key] = rest[:sp]
		rest = rest[sp:]
	}
	return out
}

func hasValidSkipDirective(fd *ast.FuncDecl) bool {
	d, err := parseSkipDirective(fd)
	return err == nil && d != nil
}
