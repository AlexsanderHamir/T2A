package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestHTTP_create_rejects_unknown_field(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"x","nope":1,"priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_list_bad_limit(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks?limit=999")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out jsonErrorBody
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "limit must be integer 0..200" {
		t.Fatalf("error %q", out.Error)
	}
}

func TestHTTP_get_task_rejects_overlong_path_id(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	long := strings.Repeat("a", maxTaskPathIDBytes+1)
	res, err := http.Get(srv.URL + "/tasks/" + long)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong id: status %d want %d", res.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTP_list_query_validation_error_messages(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	getErr := func(rawURL string) string {
		t.Helper()
		res, err := http.Get(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(res.Body)
		if cerr := res.Body.Close(); cerr != nil {
			t.Fatal(cerr)
		}
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d body %s", rawURL, res.StatusCode, b)
		}
		var out jsonErrorBody
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out.Error
	}

	base := srv.URL + "/tasks"
	if got := getErr(base + "?limit=999"); got != "limit must be integer 0..200" {
		t.Fatalf("limit 999: %q", got)
	}
	if got := getErr(base + "?limit=nope"); got != "limit must be integer 0..200" {
		t.Fatalf("limit nope: %q", got)
	}
	if got := getErr(base + "?offset=-1"); got != "offset must be non-negative integer" {
		t.Fatalf("offset -1: %q", got)
	}
	id := "11111111-1111-4111-8111-111111111111"
	if got := getErr(base + "?after_id=" + id + "&offset=0"); got != "offset cannot be used with after_id" {
		t.Fatalf("after_id+offset: %q", got)
	}
	if got := getErr(base + "?after_id=not-a-uuid"); got != "after_id must be a UUID" {
		t.Fatalf("bad uuid: %q", got)
	}
	long := strings.Repeat("1", maxListIntQueryParamBytes+1)
	if got := getErr(base + "?limit=" + long); got != "limit value too long" {
		t.Fatalf("long limit: %q", got)
	}
	if got := getErr(base + "?offset=" + long); got != "offset value too long" {
		t.Fatalf("long offset: %q", got)
	}
	longAfter := strings.Repeat("a", maxListAfterIDParamBytes+1)
	if got := getErr(base + "?after_id=" + longAfter); got != "after_id too long" {
		t.Fatalf("long after_id: %q", got)
	}
}

func TestHTTP_create_rejects_empty_title(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"   ","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_rejects_invalid_status(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{"title":"ok","status":"not_a_real_status","priority":"medium"}`
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_rejects_invalid_task_type(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{"title":"ok","priority":"medium","task_type":"not_a_real_type"}`
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_evaluate_rejects_invalid_task_type(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	body := `{"id":"draft-x","title":"ok","priority":"medium","task_type":"not_a_real_type"}`
	res, err := http.Post(srv.URL+"/tasks/evaluate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_patch_rejects_invalid_task_type(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	createRes, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"task","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	createBody, err := io.ReadAll(createRes.Body)
	if cerr := createRes.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", createRes.StatusCode, createBody)
	}
	var created domain.Task
	if err := json.Unmarshal(createBody, &created); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID, strings.NewReader(`{"task_type":"not_a_real_type"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_create_rejects_missing_priority(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}
