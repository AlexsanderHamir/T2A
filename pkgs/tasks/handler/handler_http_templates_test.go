package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func templateSaveBody(title string) string {
	return `{"name":"` + title + `","payload":` + withCreateChecklist(`{"title":"`+title+`","priority":"medium"}`) + `}`
}

func TestHTTP_task_templates_crud(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	saveRes, err := http.Post(srv.URL+"/task-templates", "application/json", strings.NewReader(templateSaveBody("Template one")))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(saveRes.Body)
	_ = saveRes.Body.Close()
	if saveRes.StatusCode != http.StatusCreated {
		t.Fatalf("save status %d body %s", saveRes.StatusCode, body)
	}
	var saved struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Fatal("missing template id")
	}
	if saved.Name != "Template one" {
		t.Fatalf("name %q", saved.Name)
	}

	listRes, err := http.Get(srv.URL + "/task-templates")
	if err != nil {
		t.Fatal(err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", listRes.StatusCode)
	}

	getRes, err := http.Get(srv.URL + "/task-templates/" + saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	getBody, _ := io.ReadAll(getRes.Body)
	_ = getRes.Body.Close()
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("get status %d body %s", getRes.StatusCode, getBody)
	}

	patchReq, _ := http.NewRequest(http.MethodPatch, srv.URL+"/task-templates/"+saved.ID, strings.NewReader(`{"name":"Renamed"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatal(err)
	}
	patchBody, _ := io.ReadAll(patchRes.Body)
	_ = patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("patch status %d body %s", patchRes.StatusCode, patchBody)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/task-templates/"+saved.ID, nil)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delRes.Body.Close()
	if delRes.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status %d", delRes.StatusCode)
	}
}

func TestHTTP_task_templates_search_q(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()
	payload := []byte(withCreateChecklist(`{"title":"Alpha task","priority":"medium"}`))
	if _, err := st.SaveTemplate(ctx, "", "Alpha task", payload); err != nil {
		t.Fatal(err)
	}
	payload2 := []byte(withCreateChecklist(`{"title":"Beta task","priority":"medium"}`))
	if _, err := st.SaveTemplate(ctx, "", "Beta task", payload2); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(srv.URL + "/task-templates?q=alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Templates []struct {
			Name string `json:"name"`
		} `json:"templates"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Templates) != 1 {
		t.Fatalf("got %d templates want 1", len(body.Templates))
	}
	if body.Templates[0].Name != "Alpha task" {
		t.Fatalf("name %q", body.Templates[0].Name)
	}
}

func TestHTTP_task_templates_instantiate(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()
	past := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	payload := []byte(withCreateChecklist(`{
		"title":"From template",
		"priority":"medium",
		"pickup_not_before":"` + past + `",
		"depends_on":[{"task_id":"00000000-0000-4000-8000-000000000001","type":"finish_to_start"}]
	}`))
	tmpl, err := st.SaveTemplate(ctx, "", "From template", payload)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Post(srv.URL+"/task-templates/instantiate", "application/json",
		strings.NewReader(`{"template_ids":["`+tmpl.ID+`","missing-id"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var body struct {
		Tasks []struct {
			Title           string  `json:"title"`
			PickupNotBefore *string `json:"pickup_not_before"`
		} `json:"tasks"`
		Errors []struct {
			TemplateID string `json:"template_id"`
			Error      string `json:"error"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Tasks) != 1 {
		t.Fatalf("tasks %d want 1", len(body.Tasks))
	}
	if body.Tasks[0].Title != "From template" {
		t.Fatalf("title %q", body.Tasks[0].Title)
	}
	if body.Tasks[0].PickupNotBefore != nil {
		t.Fatalf("past pickup_not_before should be omitted, got %v", body.Tasks[0].PickupNotBefore)
	}
	if len(body.Errors) != 1 || body.Errors[0].TemplateID != "missing-id" {
		t.Fatalf("errors %+v", body.Errors)
	}
}

func TestHTTP_task_templates_instantiate_empty_ids(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	res, err := http.Post(srv.URL+"/task-templates/instantiate", "application/json",
		strings.NewReader(`{"template_ids":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", res.StatusCode)
	}
}

func TestHTTP_task_templates_save_requires_valid_payload(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	res, err := http.Post(srv.URL+"/task-templates", "application/json",
		strings.NewReader(`{"payload":{"title":"   ","priority":"medium","checklist_items":[{"text":"x"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
}
