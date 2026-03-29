package postgres

import (
	"errors"
	"testing"
)

func TestOpen_rejectsEmptyDSN(t *testing.T) {
	_, err := Open("", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errEmptyDSN) {
		t.Fatalf("expected wrapped errEmptyDSN, got %v", err)
	}
}

func TestOpen_rejectsWhitespaceOnlyDSN(t *testing.T) {
	_, err := Open("   \t  ", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
