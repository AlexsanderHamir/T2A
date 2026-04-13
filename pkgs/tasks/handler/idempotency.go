package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/singleflight"
)

const (
	idempotencyHeader            = "Idempotency-Key"
	maxIdempotencyKeyLen         = 128
	maxIdempotencyBodySize       = 1 << 20 // 1 MiB
	defaultIdempotencyTTL        = 24 * time.Hour
	idempotencyPruneMod          = 256
	defaultIdempotencyMaxEntries = 2048
	defaultIdempotencyMaxBytes   = 8 << 20 // 8 MiB
)

var (
	taskapiHTTPIdempotentReplayTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "http_idempotent_replay_total",
		Help:      "Responses served from idempotency cache (same key, method, path, and body hash as first successful response).",
	})
)

type idempotencyPreparedRequest struct {
	compositeKey string
}

var idempSF singleflight.Group

func idempotencyTTLConfigured() time.Duration {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyTTLConfigured")
	s := strings.TrimSpace(os.Getenv("T2A_IDEMPOTENCY_TTL"))
	if s == "" {
		return defaultIdempotencyTTL
	}
	if s == "0" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return defaultIdempotencyTTL
	}
	return d
}

func idempotencyMaxEntriesConfigured() int {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyMaxEntriesConfigured")
	s := strings.TrimSpace(os.Getenv("T2A_IDEMPOTENCY_MAX_ENTRIES"))
	if s == "" {
		return defaultIdempotencyMaxEntries
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultIdempotencyMaxEntries
	}
	return n
}

func idempotencyMaxBytesConfigured() int {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyMaxBytesConfigured")
	s := strings.TrimSpace(os.Getenv("T2A_IDEMPOTENCY_MAX_BYTES"))
	if s == "" {
		return defaultIdempotencyMaxBytes
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultIdempotencyMaxBytes
	}
	return n
}

// IdempotencyTTL returns the effective in-process idempotency cache TTL from
// T2A_IDEMPOTENCY_TTL (same as WithIdempotency): default 24h, 0 disables caching.
func IdempotencyTTL() time.Duration {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.IdempotencyTTL")
	return idempotencyTTLConfigured()
}

// IdempotencyCacheLimits returns effective in-process idempotency cache limits.
// 0 means disabled for the respective bound.
func IdempotencyCacheLimits() (maxEntries int, maxBytes int) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.IdempotencyCacheLimits")
	return idempotencyMaxEntriesConfigured(), idempotencyMaxBytesConfigured()
}

func idempotencyMutatingMethod(method string) bool {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyMutatingMethod")
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func shouldCacheIdempotentStatus(code int) bool {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.shouldCacheIdempotentStatus")
	switch code {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return true
	default:
		return false
	}
}

func cloneIdempotentResponseHeaders(src http.Header) http.Header {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.cloneIdempotentResponseHeaders")
	dst := make(http.Header)
	for _, k := range []string{"Content-Type", "X-Content-Type-Options"} {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
	return dst
}

func captureIdempotentResponse(rec *httptest.ResponseRecorder) idempotencyCaptured {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.captureIdempotentResponse")
	status := rec.Code
	if status == 0 {
		status = http.StatusOK
	}
	hdr := cloneIdempotentResponseHeaders(rec.Header())
	body := rec.Body.Bytes()
	b := make([]byte, len(body))
	copy(b, body)
	return idempotencyCaptured{status: status, headers: hdr, body: b}
}

func replayIdempotentResponse(w http.ResponseWriter, cap idempotencyCaptured) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.replayIdempotentResponse")
	setAPISecurityHeaders(w)
	if v := cap.headers.Get("Content-Type"); v != "" {
		w.Header().Set("Content-Type", v)
	}
	w.WriteHeader(cap.status)
	if len(cap.body) > 0 {
		_, _ = w.Write(cap.body)
	}
}

func validateIdempotencyKey(rawKey string, w http.ResponseWriter, r *http.Request) bool {
	if len(rawKey) <= maxIdempotencyKeyLen {
		return true
	}
	slog.Log(r.Context(), slog.LevelWarn, "idempotency key too long",
		"cmd", httpLogCmd, "operation", "handler.idempotency",
		"max_len", maxIdempotencyKeyLen, "len", len(rawKey))
	writeJSONError(w, r, "idempotency.key", http.StatusBadRequest, "idempotency key too long")
	return false
}

func bodyFingerprintFromRequest(r *http.Request, w http.ResponseWriter) (string, bool) {
	if r.ContentLength < 0 {
		writeJSONError(w, r, "idempotency.content_length", http.StatusBadRequest, "idempotency requires known content length")
		return "", false
	}
	if r.ContentLength > maxIdempotencyBodySize {
		writeJSONError(w, r, "idempotency.body_too_large", http.StatusRequestEntityTooLarge, "request body too large for idempotency")
		return "", false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Log(r.Context(), slog.LevelWarn, "idempotency body read failed",
			"cmd", httpLogCmd, "operation", "handler.idempotency", "err", err)
		writeJSONError(w, r, "idempotency.read_body", http.StatusBadRequest, "could not read request body")
		return "", false
	}
	_ = r.Body.Close()
	sum := sha256.Sum256(body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	return hex.EncodeToString(sum[:]), true
}

func prepareIdempotencyRequest(r *http.Request, w http.ResponseWriter) (idempotencyPreparedRequest, bool) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.prepareIdempotencyRequest")
	rawKey := strings.TrimSpace(r.Header.Get(idempotencyHeader))
	if rawKey == "" {
		return idempotencyPreparedRequest{}, false
	}
	if !validateIdempotencyKey(rawKey, w, r) {
		return idempotencyPreparedRequest{}, false
	}
	var bodyFP string
	if r.Method == http.MethodPost || r.Method == http.MethodPatch {
		fp, ok := bodyFingerprintFromRequest(r, w)
		if !ok {
			return idempotencyPreparedRequest{}, false
		}
		bodyFP = fp
	}
	composite := r.Method + "\x00" + r.URL.Path + "\x00" + rawKey + "\x00" + bodyFP
	return idempotencyPreparedRequest{compositeKey: composite}, true
}

// WithIdempotency deduplicates mutating requests that send a non-empty Idempotency-Key header.
// The composite key is method, URL path, trimmed key, and SHA-256 of the body for
// POST/PATCH (DELETE omits a body fingerprint). Only responses with status 200, 201, or 204 are
// cached. Concurrent identical requests share one handler execution via singleflight.
//
// POST/PATCH with unknown Content-Length (chunked) are rejected with 400 because
// body fingerprinting would be ambiguous; bodies larger than 1 MiB are rejected with 413.
//
// Cache TTL comes from T2A_IDEMPOTENCY_TTL (Go duration; default 24h). Set to 0 to disable.
// The cache is in-process only and is not shared across replicas.
func WithIdempotency(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithIdempotency")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ttl := idempotencyTTLConfigured()
		if ttl <= 0 || !idempotencyMutatingMethod(r.Method) {
			h.ServeHTTP(w, r)
			return
		}
		prepared, ok := prepareIdempotencyRequest(r, w)
		if !ok {
			if strings.TrimSpace(r.Header.Get(idempotencyHeader)) == "" {
				h.ServeHTTP(w, r)
			}
			return
		}
		composite := prepared.compositeKey

		if cap, ok := idempCache.get(composite); ok {
			taskapiHTTPIdempotentReplayTotal.Inc()
			replayIdempotentResponse(w, cap)
			return
		}

		v, err, _ := idempSF.Do(composite, func() (interface{}, error) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			cap := captureIdempotentResponse(rec)
			if shouldCacheIdempotentStatus(cap.status) {
				idempCache.set(r.Context(), composite, cap, time.Now().Add(ttl))
			}
			return cap, nil
		})
		if err != nil {
			slog.Log(r.Context(), slog.LevelError, "idempotency singleflight error",
				"cmd", httpLogCmd, "operation", "handler.idempotency", "err", err)
			writeJSONError(w, r, "idempotency", http.StatusInternalServerError, "internal server error")
			return
		}
		cap := v.(idempotencyCaptured)
		replayIdempotentResponse(w, cap)
	})
}
