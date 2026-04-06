package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestParseTaskPathID(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := parseTaskPathID("   ")
		if err == nil || !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("too_long", func(t *testing.T) {
		_, err := parseTaskPathID(strings.Repeat("a", maxTaskPathIDBytes+1))
		if err == nil || !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("ok_trimmed", func(t *testing.T) {
		id, err := parseTaskPathID("  " + strings.Repeat("c", 36) + "  ")
		if err != nil || id != strings.Repeat("c", 36) {
			t.Fatalf("got %q %v", id, err)
		}
	})
}

func TestParseTaskPathItemID(t *testing.T) {
	_, err := parseTaskPathItemID(strings.Repeat("x", maxTaskPathIDBytes+1))
	if err == nil || !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}
