package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestHTTP_patch_task_event_user_response(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
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
	if err := st.AppendTaskEvent(ctx, created.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	reqBadType, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/1", strings.NewReader(`{"user_response":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqBadType.Header.Set("Content-Type", "application/json")
	resBadType, err := http.DefaultClient.Do(reqBadType)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := resBadType.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if resBadType.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong event type want 400, got %d", resBadType.StatusCode)
	}

	reqAgent, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/2", strings.NewReader(`{"user_response":"Please confirm"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqAgent.Header.Set("Content-Type", "application/json")
	reqAgent.Header.Set("X-Actor", "agent")
	resAgent, err := http.DefaultClient.Do(reqAgent)
	if err != nil {
		t.Fatal(err)
	}
	agentBody, err := io.ReadAll(resAgent.Body)
	if cerr := resAgent.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if resAgent.StatusCode != http.StatusOK {
		t.Fatalf("agent patch want 200, got %d %s", resAgent.StatusCode, agentBody)
	}
	var agentOut struct {
		ResponseThread []struct {
			By   string `json:"by"`
			Body string `json:"body"`
		} `json:"response_thread"`
	}
	if err := json.Unmarshal(agentBody, &agentOut); err != nil {
		t.Fatal(err)
	}
	if len(agentOut.ResponseThread) != 1 || agentOut.ResponseThread[0].By != "agent" || agentOut.ResponseThread[0].Body != "Please confirm" {
		t.Fatalf("agent thread %#v", agentOut.ResponseThread)
	}

	reqOK, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/2", strings.NewReader(`{"user_response":"LGTM"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqOK.Header.Set("Content-Type", "application/json")
	resOK, err := http.DefaultClient.Do(reqOK)
	if err != nil {
		t.Fatal(err)
	}
	defer resOK.Body.Close()
	if resOK.StatusCode != http.StatusOK {
		tBody, _ := io.ReadAll(resOK.Body)
		t.Fatalf("patch %d %s", resOK.StatusCode, tBody)
	}
	var one struct {
		Seq            int64      `json:"seq"`
		UserResponse   *string    `json:"user_response"`
		UserResponseAt *time.Time `json:"user_response_at"`
		ResponseThread []struct {
			By   string `json:"by"`
			Body string `json:"body"`
		} `json:"response_thread"`
	}
	if err := json.NewDecoder(resOK.Body).Decode(&one); err != nil {
		t.Fatal(err)
	}
	if one.Seq != 2 || one.UserResponse == nil || *one.UserResponse != "LGTM" {
		t.Fatalf("payload %#v", one)
	}
	if one.UserResponseAt == nil {
		t.Fatal("expected user_response_at on PATCH response")
	}
	if len(one.ResponseThread) != 2 {
		t.Fatalf("want 2 thread entries, got %#v", one.ResponseThread)
	}

	resList, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resList.Body.Close()
	var listPayload struct {
		Events []struct {
			Seq            int64   `json:"seq"`
			UserResponse   *string `json:"user_response"`
			ResponseThread []struct {
				By   string `json:"by"`
				Body string `json:"body"`
			} `json:"response_thread"`
		} `json:"events"`
	}
	if err := json.NewDecoder(resList.Body).Decode(&listPayload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range listPayload.Events {
		if e.Seq == 2 && e.UserResponse != nil && *e.UserResponse == "LGTM" && len(e.ResponseThread) == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list missing user_response or thread: %#v", listPayload.Events)
	}
}

func TestHTTP_get_task_event(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
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

	resOK, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resOK.Body.Close()
	if resOK.StatusCode != http.StatusOK {
		tBody, _ := io.ReadAll(resOK.Body)
		t.Fatalf("event status %d %s", resOK.StatusCode, tBody)
	}
	var one struct {
		TaskID string `json:"task_id"`
		Seq    int64  `json:"seq"`
		Type   string `json:"type"`
	}
	if err := json.NewDecoder(resOK.Body).Decode(&one); err != nil {
		t.Fatal(err)
	}
	if one.TaskID != created.ID || one.Seq != 1 || one.Type != string(domain.EventTaskCreated) {
		t.Fatalf("payload %#v", one)
	}

	res404, err := http.Get(srv.URL + "/tasks/" + created.ID + "/events/99")
	if err != nil {
		t.Fatal(err)
	}
	defer res404.Body.Close()
	if res404.StatusCode != http.StatusNotFound {
		t.Fatalf("missing seq want 404, got %d", res404.StatusCode)
	}

	assertEventSeq400 := func(t *testing.T, rawURL, wantMsg string) {
		t.Helper()
		res, err := http.Get(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
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
		if out.Error != wantMsg {
			t.Fatalf("%s: error %q want %q", rawURL, out.Error, wantMsg)
		}
	}

	assertEventSeq400(t, srv.URL+"/tasks/"+created.ID+"/events/0", "seq must be a positive integer")
	assertEventSeq400(t, srv.URL+"/tasks/"+created.ID+"/events/nope", "seq must be a positive integer")

	longSeq := strings.Repeat("1", maxTaskEventSeqParamBytes+1)
	assertEventSeq400(t, srv.URL+"/tasks/"+created.ID+"/events/"+longSeq, "seq too long")
}

func TestHTTP_task_events_query_validation_error_messages(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"e","priority":"medium"}`))
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

	getErr := func(rawURL string) string {
		t.Helper()
		res, err := http.Get(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		if cerr := res.Body.Close(); cerr != nil {
			t.Fatal(cerr)
		}
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d body %s", rawURL, res.StatusCode, body)
		}
		var out jsonErrorBody
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		return out.Error
	}

	base := srv.URL + "/tasks/" + created.ID + "/events"
	long := strings.Repeat("1", maxTaskEventSeqParamBytes+1)

	if got := getErr(base + "?limit=5&offset=0"); got != "offset is not supported for task events; use before_seq or after_seq" {
		t.Fatalf("offset: %q", got)
	}
	if got := getErr(base + "?limit=5&before_seq=1&after_seq=1"); got != "before_seq and after_seq cannot both be set" {
		t.Fatalf("both cursors: %q", got)
	}
	if got := getErr(base + "?limit=5&before_seq=" + long); got != "before_seq or after_seq too long" {
		t.Fatalf("long before_seq: %q", got)
	}
	if got := getErr(base + "?limit=5&after_seq=" + long); got != "before_seq or after_seq too long" {
		t.Fatalf("long after_seq: %q", got)
	}
	if got := getErr(base + "?limit=" + long); got != "limit too long" {
		t.Fatalf("long limit: %q", got)
	}
	if got := getErr(base + "?limit=999"); got != "limit must be integer 0..200" {
		t.Fatalf("limit 999: %q", got)
	}
	if got := getErr(base + "?limit=nope&before_seq=1"); got != "limit must be integer 0..200" {
		t.Fatalf("limit nope: %q", got)
	}
	if got := getErr(base + "?limit=10&before_seq=0"); got != "before_seq must be a positive integer" {
		t.Fatalf("before_seq 0: %q", got)
	}
	if got := getErr(base + "?limit=10&after_seq=-1"); got != "after_seq must be a positive integer" {
		t.Fatalf("after_seq -1: %q", got)
	}
	if got := getErr(base + "?limit=10&before_seq=nope"); got != "before_seq must be a positive integer" {
		t.Fatalf("before_seq nope: %q", got)
	}
	if got := getErr(base + "?limit=10&after_seq=xyz"); got != "after_seq must be a positive integer" {
		t.Fatalf("after_seq xyz: %q", got)
	}
}

func TestHTTP_patch_task_event_rejects_overlong_seq(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	ctx := context.Background()

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"hello","priority":"medium"}`))
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
	if err := st.AppendTaskEvent(ctx, created.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	longSeq := strings.Repeat("1", maxTaskEventSeqParamBytes+1)
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/tasks/"+created.ID+"/events/"+longSeq, strings.NewReader(`{"user_response":"x"}`))
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
		t.Fatalf("patch overlong seq want 400, got %d %s", resPatch.StatusCode, patchBody)
	}
	var errOut jsonErrorBody
	if err := json.Unmarshal(patchBody, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Error != "seq too long" {
		t.Fatalf("patch overlong seq error %q", errOut.Error)
	}
}
