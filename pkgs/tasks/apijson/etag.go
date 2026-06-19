package apijson

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// RevalidatableCacheControl is the Cache-Control value used by JSON GET
// endpoints that participate in ETag/304 conditional revalidation. The
// "no-cache" directive forces the browser to send If-None-Match on every
// request (rather than serving from cache without asking), while
// "must-revalidate" forbids stale serving when the network fails.
// "private" keeps shared caches (corporate proxies) from caching at all
// since responses may contain per-user task data.
const RevalidatableCacheControl = "private, no-cache, must-revalidate"

// etagPrefixBytes is the number of SHA-256 bytes we keep in the ETag.
// 16 bytes (128 bits) is well past the birthday bound for any realistic
// number of revisions of a single resource, and yields a short header.
const etagPrefixBytes = 16

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ApplyRevalidatableHeaders writes the same hardening as ApplySecurityHeaders
// but swaps Cache-Control: no-store for the revalidatable directive so
// callers can serve 304 on If-None-Match without the browser refusing to
// keep the cached body.
func ApplyRevalidatableHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", RevalidatableCacheControl)
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ComputeETag returns a strong-form ETag header value for body, using a
// SHA-256 prefix. The result includes the surrounding double quotes per
// RFC 7232 §2.3 and is safe to pass directly to Header().Set("ETag", ...).
func ComputeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:etagPrefixBytes]) + `"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// IfNoneMatchMatches reports whether the client's If-None-Match header
// (possibly a comma-separated list, possibly "*") would be satisfied by
// the given strong ETag value. It treats weak validators ("W/...") as
// equivalent for the purpose of revalidation; we never emit weak tags
// ourselves but the spec encourages tolerant comparison on the receive
// side. Returns false for an empty header or when no entry matches.
func IfNoneMatchMatches(headerValue, etag string) bool {
	if headerValue == "" || etag == "" {
		return false
	}
	for _, raw := range strings.Split(headerValue, ",") {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			return true
		}
		if strings.HasPrefix(candidate, "W/") {
			candidate = strings.TrimPrefix(candidate, "W/")
		}
		if candidate == etag {
			return true
		}
	}
	return false
}
