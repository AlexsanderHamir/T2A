package handlertest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/httpsecurityexpect"
	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestHTTP_health(t *testing.T) {
	srv := NewServer(t)
	defer srv.Close()

	for _, path := range []string{"/health", "/health/live"} {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d", res.StatusCode)
			}
			var body struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			}
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Status != "ok" {
				t.Fatalf("status field %q", body.Status)
			}
			if body.Version == "" {
				t.Fatal("missing version")
			}
		})
	}
}

func TestHTTP_health_ready_ok(t *testing.T) {
	srv := NewServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Status  string            `json:"status"`
		Checks  map[string]string `json:"checks"`
		Version string            `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Checks["database"] != "ok" || body.Version == "" {
		t.Fatalf("body %+v", body)
	}
}

func TestHTTP_metrics_scrape(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	api := handler.WithRecovery(handler.WithHTTPMetrics(handler.WithAccessLog(handler.NewHandler(store.NewStore(db), handler.NewSSEHub(), nil))))
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", handler.WrapPrometheusHandler(promhttp.Handler()))
	mux.Handle("/", api)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resTasks, err := http.Get(srv.URL + "/tasks")
	if err != nil {
		t.Fatal(err)
	}
	_ = resTasks.Body.Close()

	res, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	httpsecurityexpect.AssertBaselineHeaders(t, res.Header)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("taskapi_http_requests_total")) {
		t.Fatalf("metrics body missing taskapi_http_requests_total")
	}
}

func TestHTTP_health_ready_degraded_when_db_closed(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(handler.NewHandler(st, handler.NewSSEHub(), nil))
	defer srv.Close()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Status  string            `json:"status"`
		Checks  map[string]string `json:"checks"`
		Version string            `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "degraded" || body.Checks["database"] != "fail" || body.Version == "" {
		t.Fatalf("body %+v", body)
	}
}

func TestHTTP_health_ready_workspace_repo_ok(t *testing.T) {
	root := t.TempDir()
	srv := NewServerWithRepo(t, root)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Status  string            `json:"status"`
		Checks  map[string]string `json:"checks"`
		Version string            `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Checks["database"] != "ok" || body.Checks["workspace_repo"] != "ok" || body.Version == "" {
		t.Fatalf("body %+v", body)
	}
}

func TestHTTP_health_ready_workspace_repo_fail_when_root_removed(t *testing.T) {
	root := t.TempDir()
	db := tasktestdb.OpenSQLite(t)
	rep, err := repo.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler.NewHandler(store.NewStore(db), handler.NewSSEHub(), rep))
	defer srv.Close()

	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Status  string            `json:"status"`
		Checks  map[string]string `json:"checks"`
		Version string            `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "degraded" || body.Checks["database"] != "ok" || body.Checks["workspace_repo"] != "fail" || body.Version == "" {
		t.Fatalf("body %+v", body)
	}
}

func TestHTTP_health_includes_security_headers(t *testing.T) {
	srv := NewServer(t)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	httpsecurityexpect.AssertBaselineHeaders(t, res.Header)
}
