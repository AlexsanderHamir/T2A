package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutConfigured(t *testing.T) {
	t.Setenv(requestTimeoutEnv, "")
	if got := RequestTimeout(); got != defaultRequestTimeout {
		t.Fatalf("default got %v", got)
	}
	t.Setenv(requestTimeoutEnv, "12s")
	if got := RequestTimeout(); got != 12*time.Second {
		t.Fatalf("configured got %v", got)
	}
	t.Setenv(requestTimeoutEnv, "0")
	if got := RequestTimeout(); got != 0 {
		t.Fatalf("zero got %v", got)
	}
	t.Setenv(requestTimeoutEnv, "bad")
	if got := RequestTimeout(); got != defaultRequestTimeout {
		t.Fatalf("invalid fallback got %v", got)
	}
}

func TestWithRequestTimeoutSetsDeadline(t *testing.T) {
	t.Setenv(requestTimeoutEnv, "2s")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected context deadline")
		}
		w.WriteHeader(http.StatusOK)
	})
	h := WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithRequestTimeoutSkipsEventsSSE(t *testing.T) {
	t.Setenv(requestTimeoutEnv, "2s")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); ok {
			t.Fatal("did not expect deadline for /events")
		}
		w.WriteHeader(http.StatusOK)
	})
	h := WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithRequestTimeoutDisabledNoDeadline(t *testing.T) {
	t.Setenv(requestTimeoutEnv, "0")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); ok {
			t.Fatal("did not expect deadline when disabled")
		}
		w.WriteHeader(http.StatusOK)
	})
	h := WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithRequestTimeoutContextCanceled(t *testing.T) {
	t.Setenv(requestTimeoutEnv, "1ms")
	done := make(chan struct{})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		if r.Context().Err() != context.DeadlineExceeded {
			t.Fatalf("ctx err %v", r.Context().Err())
		}
		close(done)
		w.WriteHeader(http.StatusOK)
	})
	h := WithRequestTimeout(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	h.ServeHTTP(rec, req)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for context cancellation")
	}
}
