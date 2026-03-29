package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRecovery_Returns500JSONOnPanic(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("intentional test panic")
	})
	srv := httptest.NewServer(WithRecovery(h))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "internal server error" {
		t.Fatalf("error body %q", body.Error)
	}
}
