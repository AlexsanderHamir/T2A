package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestMaxRequestBodyBytesConfigured(t *testing.T) {
	t.Setenv(maxRequestBodyEnv, "")
	if MaxRequestBodyBytesConfigured() != 0 {
		t.Fatalf("unset want 0")
	}
	t.Setenv(maxRequestBodyEnv, "4096")
	if MaxRequestBodyBytesConfigured() != 4096 {
		t.Fatalf("4096")
	}
	t.Setenv(maxRequestBodyEnv, "0")
	if MaxRequestBodyBytesConfigured() != 0 {
		t.Fatalf("zero means unlimited")
	}
	t.Setenv(maxRequestBodyEnv, "-3")
	if MaxRequestBodyBytesConfigured() != 0 {
		t.Fatalf("negative -> unlimited")
	}
	t.Setenv(maxRequestBodyEnv, "nope")
	if MaxRequestBodyBytesConfigured() != 0 {
		t.Fatalf("invalid -> unlimited")
	}
}

func TestHTTP_max_body_rejects_content_length_over_limit(t *testing.T) {
	t.Setenv(maxRequestBodyEnv, "50")
	db := testdb.OpenSQLite(t)
	srv := httptest.NewServer(WithMaxRequestBody(NewHandler(store.NewStore(db), NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	body := `{"title":"` + strings.Repeat("h", 40) + `","priority":"medium"}`
	if len(body) <= 50 {
		t.Fatal("body should exceed limit")
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var errBody jsonErrorBody
	if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error != "request body too large" {
		t.Fatalf("msg %q", errBody.Error)
	}
}

func TestHTTP_max_body_allows_under_limit(t *testing.T) {
	t.Setenv(maxRequestBodyEnv, "4096")
	db := testdb.OpenSQLite(t)
	srv := httptest.NewServer(WithMaxRequestBody(NewHandler(store.NewStore(db), NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	body := `{"title":"ok","priority":"medium"}`
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d %s", res.StatusCode, b)
	}
}

func TestHTTP_max_body_unknown_content_length_still_bounded(t *testing.T) {
	t.Setenv(maxRequestBodyEnv, "48")
	db := testdb.OpenSQLite(t)
	h := WithMaxRequestBody(NewHandler(store.NewStore(db), NewSSEHub(), nil))

	// Valid create JSON > 48 bytes; ContentLength -1 so middleware cannot pre-reject on length.
	body := `{"title":"` + strings.Repeat("x", 40) + `","priority":"medium"}`
	if len(body) <= 48 {
		t.Fatalf("body len %d need >48", len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/tasks", io.NopCloser(strings.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	req.Host = "example.com"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
