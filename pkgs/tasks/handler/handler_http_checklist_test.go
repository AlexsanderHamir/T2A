package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestHTTP_patch_checklist_item_text_updates_and_returns_items(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
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
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}
	it, err := st.AddChecklistItem(ctx, created.ID, "alpha", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"text":"beta"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	patchBody, err := io.ReadAll(resPatch.Body)
	if cerr := resPatch.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resPatch.StatusCode != http.StatusOK {
		t.Fatalf("patch %d %s", resPatch.StatusCode, patchBody)
	}
	var out struct {
		Items []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"items"`
	}
	if err := json.Unmarshal(patchBody, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Text != "beta" {
		t.Fatalf("items %#v", out.Items)
	}
}

func TestHTTP_patch_checklist_item_done_rejects_default_user_actor(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
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
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}
	it, err := st.AddChecklistItem(ctx, created.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"done":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resPatch.Body.Close()
	if resPatch.StatusCode != http.StatusBadRequest {
		t.Fatalf("user done patch want 400, got %d", resPatch.StatusCode)
	}
	patchBody, err := io.ReadAll(resPatch.Body)
	if err != nil {
		t.Fatal(err)
	}
	var errOut jsonErrorBody
	if err := json.Unmarshal(patchBody, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Error != "only the agent may mark checklist items done or undone" {
		t.Fatalf("error %q", errOut.Error)
	}
}

func TestHTTP_patch_checklist_item_rejects_text_and_done_together(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}
	it, err := st.AddChecklistItem(ctx, created.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"text":"x","done":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resPatch.Body.Close()
	if resPatch.StatusCode != http.StatusBadRequest {
		t.Fatalf("both fields want 400, got %d", resPatch.StatusCode)
	}
	patchBody, err := io.ReadAll(resPatch.Body)
	if err != nil {
		t.Fatal(err)
	}
	var errOut jsonErrorBody
	if err := json.Unmarshal(patchBody, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Error != "send exactly one of text or done" {
		t.Fatalf("error %q", errOut.Error)
	}
}

func TestHTTP_patch_checklist_item_rejects_empty_trimmed_text(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"chk","priority":"medium"}`))
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
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created domain.Task
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}
	it, err := st.AddChecklistItem(ctx, created.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+created.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"text":"   "}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resPatch.Body.Close()
	if resPatch.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty text want 400, got %d", resPatch.StatusCode)
	}
	patchBody, err := io.ReadAll(resPatch.Body)
	if err != nil {
		t.Fatal(err)
	}
	var errOut jsonErrorBody
	if err := json.Unmarshal(patchBody, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Error != "text required" {
		t.Fatalf("error %q", errOut.Error)
	}
}

func TestHTTP_patch_checklist_item_rejects_text_when_checklist_inherit(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"par","priority":"medium"}`))
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
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("parent create %d %s", res.StatusCode, b)
	}
	var parent domain.Task
	if err := json.Unmarshal(b, &parent); err != nil {
		t.Fatal(err)
	}

	resItem, err := http.Post(srv.URL+"/tasks/"+parent.ID+"/checklist/items", "application/json", strings.NewReader(`{"text":"owned"}`))
	if err != nil {
		t.Fatal(err)
	}
	itemBody, err := io.ReadAll(resItem.Body)
	if cerr := resItem.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resItem.StatusCode != http.StatusCreated {
		t.Fatalf("add item %d %s", resItem.StatusCode, itemBody)
	}
	var it domain.TaskChecklistItem
	if err := json.Unmarshal(itemBody, &it); err != nil {
		t.Fatal(err)
	}

	childBody := `{"title":"kid","priority":"medium","parent_id":"` + parent.ID + `","checklist_inherit":true}`
	resChild, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(childBody))
	if err != nil {
		t.Fatal(err)
	}
	cb, err := io.ReadAll(resChild.Body)
	if cerr := resChild.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resChild.StatusCode != http.StatusCreated {
		t.Fatalf("child create %d %s", resChild.StatusCode, cb)
	}
	var child domain.Task
	if err := json.Unmarshal(cb, &child); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/tasks/"+child.ID+"/checklist/items/"+it.ID,
		strings.NewReader(`{"text":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resPatch, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resPatch.Body.Close()
	patchBody, err := io.ReadAll(resPatch.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resPatch.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 got %d %s", resPatch.StatusCode, patchBody)
	}
	var errOut jsonErrorBody
	if err := json.Unmarshal(patchBody, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Error != "cannot update inherited checklist definitions from this task" {
		t.Fatalf("error %q", errOut.Error)
	}
}
