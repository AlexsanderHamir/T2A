package handler

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTP_projectRoutesPublishSSE(t *testing.T) {
	srv, _, hub := newSSETriggerServer(t)
	defer srv.Close()

	ch, cancel := hub.Subscribe()
	defer cancel()
	project := postProjectJSON(t, srv, `{"name":"SSE project"}`, http.StatusCreated)
	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "POST /projects", got, []string{"project_created:" + project.ID})

	mustDoJSON(t, http.MethodPost, srv.URL+"/projects/"+project.ID+"/context",
		`{"kind":"note","title":"Pinned","body":"Context","pinned":true}`, "", http.StatusCreated)
	got = summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "POST /projects/{id}/context", got, []string{"project_context_changed:" + project.ID})

	mustDoJSON(t, http.MethodPatch, srv.URL+"/projects/"+project.ID,
		`{"description":"Updated"}`, "", http.StatusOK)
	got = summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "PATCH /projects/{id}", got, []string{"project_updated:" + project.ID})
}
