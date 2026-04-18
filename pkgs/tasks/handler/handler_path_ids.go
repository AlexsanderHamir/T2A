package handler

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// maxTaskPathIDBytes caps path segments for task UUIDs, draft ids, checklist item ids, etc.
const maxTaskPathIDBytes = 128

// maxPhaseSeqParamBytes caps the {phaseSeq} path segment so strconv work and
// log fields stay bounded. Phase sequences are small positive integers; the
// 32-byte cap matches the documented event-seq cap.
const maxPhaseSeqParamBytes = 32

func parseTaskPathID(id string) (string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseTaskPathID")
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if len(id) > maxTaskPathIDBytes {
		return "", fmt.Errorf("%w: id too long", domain.ErrInvalidInput)
	}
	return id, nil
}

func parseTaskPathItemID(id string) (string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseTaskPathItemID")
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%w: item id", domain.ErrInvalidInput)
	}
	if len(id) > maxTaskPathIDBytes {
		return "", fmt.Errorf("%w: item id too long", domain.ErrInvalidInput)
	}
	return id, nil
}

// parseTaskPathCycleID validates the {cycleId} path segment for the cycles
// resource family (same UUID-shape and 128-byte cap as task ids; the bare
// field name "cycle id" surfaces in the 400 message).
func parseTaskPathCycleID(id string) (string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseTaskPathCycleID")
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%w: cycle id", domain.ErrInvalidInput)
	}
	if len(id) > maxTaskPathIDBytes {
		return "", fmt.Errorf("%w: cycle id too long", domain.ErrInvalidInput)
	}
	return id, nil
}

// parseTaskPathPhaseSeq validates the {phaseSeq} path segment. Sequences
// are positive int64 values; whitespace, non-numeric, zero, and negative
// values are rejected.
func parseTaskPathPhaseSeq(raw string) (int64, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseTaskPathPhaseSeq")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("%w: phase_seq must be a positive integer", domain.ErrInvalidInput)
	}
	if len(raw) > maxPhaseSeqParamBytes {
		return 0, fmt.Errorf("%w: phase_seq too long", domain.ErrInvalidInput)
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: phase_seq must be a positive integer", domain.ErrInvalidInput)
	}
	return n, nil
}
