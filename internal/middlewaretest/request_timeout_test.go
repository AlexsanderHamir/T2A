package middlewaretest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

func TestRequestTimeoutConfigured(t *testing.T) {
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "")
	if got := middleware.RequestTimeout(); got != 30*time.Second {
		t.Fatalf("default got %v", got)
	}
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "12s")
	if got := middleware.RequestTimeout(); got != 12*time.Second {
		t.Fatalf("configured got %v", got)
	}
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "0")
	if got := middleware.RequestTimeout(); got != 0 {
		t.Fatalf("zero got %v", got)
	}
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "bad")
	if got := middleware.RequestTimeout(); got != 30*time.Second {
		t.Fatalf("invalid fallback got %v", got)
	}
}

func TestWithRequestTimeoutSetsDeadline(t *testing.T) {
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "2s")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected context deadline")
		}
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithRequestTimeoutSkipsEventsSSE(t *testing.T) {
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "2s")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); ok {
			t.Fatal("did not expect deadline for /events")
		}
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithRequestTimeoutDisabledNoDeadline(t *testing.T) {
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "0")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); ok {
			t.Fatal("did not expect deadline when disabled")
		}
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithRequestTimeoutContextCanceled(t *testing.T) {
	t.Setenv("T2A_HTTP_REQUEST_TIMEOUT", "1ms")
	done := make(chan struct{})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		if r.Context().Err() != context.DeadlineExceeded {
			t.Fatalf("ctx err %v", r.Context().Err())
		}
		close(done)
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	h.ServeHTTP(rec, req)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for context cancellation")
	}
}
