package handler

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"
)

const (
	defaultRateLimitPerMin = 120
	rateLimitMaxVisitors   = 10_000
	rateLimitVisitorTTL    = 15 * time.Minute
	rateLimitPruneLen      = 8_000
)

var taskapiHTTPRateLimitedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "taskapi",
	Name:      "http_rate_limited_total",
	Help:      "HTTP requests rejected with 429 (per-IP rate limit).",
})

// RateLimitPerMinuteConfigured returns the effective per-IP limit from T2A_RATE_LIMIT_PER_MIN
// (requests per minute). Unset uses defaultRateLimitPerMin. Invalid or negative values use the default.
// Zero disables rate limiting (WithRateLimit becomes a no-op).
func RateLimitPerMinuteConfigured() int {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.RateLimitPerMinuteConfigured")
	return parseRateLimitPerMinFromEnv()
}

func parseRateLimitPerMinFromEnv() int {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.parseRateLimitPerMinFromEnv")
	s := strings.TrimSpace(os.Getenv("T2A_RATE_LIMIT_PER_MIN"))
	if s == "" {
		return defaultRateLimitPerMin
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultRateLimitPerMin
	}
	return n
}

func omitRateLimit(r *http.Request) bool {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.omitRateLimit")
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/health", "/health/live", "/health/ready", "/metrics":
		return true
	default:
		return false
	}
}

func clientIPForRateLimit(r *http.Request) string {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.clientIPForRateLimit")
	addr := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

type rateLimitVisitor struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu       sync.Mutex
	perMin   int
	visitors map[string]*rateLimitVisitor
}

func newIPRateLimiter(perMin int) *ipRateLimiter {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.newIPRateLimiter")
	return &ipRateLimiter{
		perMin:   perMin,
		visitors: make(map[string]*rateLimitVisitor),
	}
}

func (il *ipRateLimiter) allow(ip string) bool {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.ipRateLimiter.allow")
	il.mu.Lock()
	defer il.mu.Unlock()
	now := time.Now()
	if len(il.visitors) > rateLimitPruneLen {
		il.pruneLocked(now)
	}
	v, ok := il.visitors[ip]
	if !ok {
		if len(il.visitors) >= rateLimitMaxVisitors {
			il.pruneLocked(now)
		}
		if len(il.visitors) >= rateLimitMaxVisitors {
			for k := range il.visitors {
				delete(il.visitors, k)
				break
			}
		}
		lim := rate.NewLimiter(rate.Limit(float64(il.perMin)/60.0), il.perMin)
		v = &rateLimitVisitor{lim: lim, lastSeen: now}
		il.visitors[ip] = v
	}
	v.lastSeen = now
	return v.lim.Allow()
}

func (il *ipRateLimiter) pruneLocked(now time.Time) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.ipRateLimiter.pruneLocked")
	for ip, v := range il.visitors {
		if now.Sub(v.lastSeen) > rateLimitVisitorTTL {
			delete(il.visitors, ip)
		}
	}
}

// WithRateLimit enforces a token-bucket limit per client IP (RemoteAddr host) using T2A_RATE_LIMIT_PER_MIN.
// Unset env uses defaultRateLimitPerMin. Set to 0 to disable (no-op wrapper).
// GET /health, /health/live, /health/ready, and /metrics are exempt.
// Rejected requests receive 429, plain text body, and Retry-After: 60.
func WithRateLimit(h http.Handler) http.Handler {
	perMin := parseRateLimitPerMinFromEnv()
	if perMin == 0 {
		return h
	}
	il := newIPRateLimiter(perMin)
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithRateLimit", "per_ip_per_min", perMin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if omitRateLimit(r) {
			h.ServeHTTP(w, r)
			return
		}
		ip := clientIPForRateLimit(r)
		if !il.allow(ip) {
			taskapiHTTPRateLimitedTotal.Inc()
			slog.Warn("rate limit exceeded", "cmd", httpLogCmd, "operation", "http.rate_limit", "client_ip", ip)
			setAPISecurityHeaders(w)
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded\n", http.StatusTooManyRequests)
			return
		}
		h.ServeHTTP(w, r)
	})
}
