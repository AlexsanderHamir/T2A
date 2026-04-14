package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func TestWithHTTPMetrics(t *testing.T) {
	t.Run("skipsHealthPaths", func(t *testing.T) {
		inner := http.NewServeMux()
		inner.Handle("GET /health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		h := WithHTTPMetrics(inner)
		for _, p := range []string{"/health", "/health/live", "/health/ready"} {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if p == "/health" && rec.Code != http.StatusOK {
				t.Fatalf("%s: %d", p, rec.Code)
			}
		}
		mfs, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, wantRoute := range []string{"GET /health", "GET /health/live", "GET /health/ready"} {
			if metricFamilyHasRouteLabel(mfs, "taskapi_http_requests_total", wantRoute) {
				t.Fatalf("unexpected metrics for skipped route %q", wantRoute)
			}
		}
	})

	t.Run("recordsRequest", func(t *testing.T) {
		inner := http.NewServeMux()
		inner.Handle("GET /hit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
		h := WithHTTPMetrics(inner)
		req := httptest.NewRequest(http.MethodGet, "/hit", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("code %d", rec.Code)
		}
		n := testutil.ToFloat64(taskapiHTTPRequestsTotal.WithLabelValues("GET", "GET /hit", "418"))
		if n != 1 {
			t.Fatalf("requests_total got %v want 1", n)
		}
	})
}

func metricFamilyHasRouteLabel(mfs []*dto.MetricFamily, familyName, routeValue string) bool {
	for _, mf := range mfs {
		if mf.GetName() != familyName {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "route" && lp.GetValue() == routeValue {
					return true
				}
			}
		}
	}
	return false
}
