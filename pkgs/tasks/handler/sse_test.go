package handler

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestSSEHub_Publish_deliversToSubscriber(t *testing.T) {
	h := NewSSEHub()
	ch, cancel := h.Subscribe()
	defer cancel()

	h.Publish(TaskChangeEvent{Type: TaskCreated, ID: "abc-123"})
	select {
	case line := <-ch:
		var ev TaskChangeEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != TaskCreated || ev.ID != "abc-123" {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSSEHub_Publish_nonBlockingSlowConsumer(t *testing.T) {
	h := NewSSEHub()
	_, cancel := h.Subscribe()
	defer cancel()
	for i := 0; i < 64; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "x"})
	}
}

func TestHTTP_SSE_receivesEventAfterCreate(t *testing.T) {
	db := testdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub())
	srv := httptest.NewServer(h)
	defer srv.Close()

	streamReady := make(chan struct{})
	payload := make(chan string, 1)
	go func() {
		res, err := http.Get(srv.URL + "/events")
		if err != nil {
			t.Error(err)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("sse status %d", res.StatusCode)
			return
		}
		br := bufio.NewReader(res.Body)
		line1, err := br.ReadString('\n')
		if err != nil {
			t.Error(err)
			return
		}
		if !strings.HasPrefix(strings.TrimSpace(line1), "retry:") {
			t.Errorf("want retry line, got %q", line1)
		}
		if _, err := br.ReadString('\n'); err != nil {
			t.Error(err)
			return
		}
		close(streamReady)
		dataLine, err := br.ReadString('\n')
		if err != nil {
			t.Error(err)
			return
		}
		s := strings.TrimSpace(dataLine)
		if !strings.HasPrefix(s, "data:") {
			t.Errorf("want data line, got %q", dataLine)
			return
		}
		payload <- strings.TrimSpace(strings.TrimPrefix(s, "data:"))
	}()

	<-streamReady
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"sse"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d", res.StatusCode)
	}

	select {
	case p := <-payload:
		var ev TaskChangeEvent
		if err := json.Unmarshal([]byte(p), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != TaskCreated {
			t.Fatalf("type %q", ev.Type)
		}
		if ev.ID == "" {
			t.Fatal("empty id")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for SSE payload")
	}
}
