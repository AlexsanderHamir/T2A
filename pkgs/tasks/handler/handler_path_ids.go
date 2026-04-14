package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// maxTaskPathIDBytes caps path segments for task UUIDs, draft ids, checklist item ids, etc.
const maxTaskPathIDBytes = 128

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
