package middlewaretest

import (
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

func TestMaxRequestBodyBytesConfigured(t *testing.T) {
	t.Setenv("T2A_MAX_REQUEST_BODY_BYTES", "")
	if middleware.MaxRequestBodyBytesConfigured() != 1<<20 {
		t.Fatalf("unset want default")
	}
	t.Setenv("T2A_MAX_REQUEST_BODY_BYTES", "4096")
	if middleware.MaxRequestBodyBytesConfigured() != 4096 {
		t.Fatalf("4096")
	}
	t.Setenv("T2A_MAX_REQUEST_BODY_BYTES", "0")
	if middleware.MaxRequestBodyBytesConfigured() != 0 {
		t.Fatalf("zero means unlimited")
	}
	t.Setenv("T2A_MAX_REQUEST_BODY_BYTES", "-3")
	if middleware.MaxRequestBodyBytesConfigured() != 1<<20 {
		t.Fatalf("negative -> default")
	}
	t.Setenv("T2A_MAX_REQUEST_BODY_BYTES", "nope")
	if middleware.MaxRequestBodyBytesConfigured() != 1<<20 {
		t.Fatalf("invalid -> default")
	}
}
