package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	taskTagPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)
	taskMilestonePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 ._-]{0,63}$`)
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidateTaskTag returns ErrInvalidInput when tag does not match the wire rules.
func ValidateTaskTag(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("%w: tag must not be empty", ErrInvalidInput)
	}
	if !taskTagPattern.MatchString(tag) {
		return fmt.Errorf("%w: invalid tag %q", ErrInvalidInput, tag)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidateTaskTags validates every tag and rejects duplicates.
func ValidateTaskTags(tags []string) error {
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if err := ValidateTaskTag(tag); err != nil {
			return err
		}
		if _, ok := seen[tag]; ok {
			return fmt.Errorf("%w: duplicate tag %q", ErrInvalidInput, tag)
		}
		seen[tag] = struct{}{}
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidateTaskMilestone returns ErrInvalidInput when milestone is non-empty but invalid.
func ValidateTaskMilestone(milestone string) error {
	milestone = strings.TrimSpace(milestone)
	if milestone == "" {
		return nil
	}
	if !taskMilestonePattern.MatchString(milestone) {
		return fmt.Errorf("%w: invalid milestone %q", ErrInvalidInput, milestone)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NormalizeTaskTags trims, drops empties, and de-duplicates while preserving order.
func NormalizeTaskTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}
