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
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
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

func TestHTTP_SSE_testMode_onCreate_emitsTaskUpdatedForFirstTaskInList(t *testing.T) {
	t.Setenv("T2A_SSE_TEST", "1")
	db := testdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resA, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resA.Body.Close()
	if resA.StatusCode != http.StatusCreated {
		t.Fatalf("create first: %d", resA.StatusCode)
	}
	var createdA struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resA.Body).Decode(&createdA); err != nil {
		t.Fatal(err)
	}
	idA := createdA.ID

	streamReady := make(chan struct{})
	payloads := make(chan string, 4)
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
		if _, err := br.ReadString('\n'); err != nil {
			t.Error(err)
			return
		}
		if _, err := br.ReadString('\n'); err != nil {
			t.Error(err)
			return
		}
		close(streamReady)
		readDataPayload := func() string {
			for {
				dataLine, err := br.ReadString('\n')
				if err != nil {
					t.Error(err)
					return ""
				}
				s := strings.TrimSpace(dataLine)
				if strings.HasPrefix(s, "data:") {
					return strings.TrimSpace(strings.TrimPrefix(s, "data:"))
				}
			}
		}
		for i := 0; i < 2; i++ {
			payloads <- readDataPayload()
		}
	}()

	<-streamReady
	resB, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"second"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resB.Body.Close()
	if resB.StatusCode != http.StatusCreated {
		t.Fatalf("create second: %d", resB.StatusCode)
	}
	var createdB struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resB.Body).Decode(&createdB); err != nil {
		t.Fatal(err)
	}
	idB := createdB.ID

	firstInList := idA
	if idB < idA {
		firstInList = idB
	}

	for i, want := range []struct {
		typ TaskChangeType
		id  string
	}{
		{TaskCreated, idB},
		{TaskUpdated, firstInList},
	} {
		var p string
		select {
		case p = <-payloads:
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for payload %d", i)
		}
		var ev TaskChangeEvent
		if err := json.Unmarshal([]byte(p), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != want.typ || ev.ID != want.id {
			t.Fatalf("payload %d: got %+v want type=%s id=%s", i, ev, want.typ, want.id)
		}
	}
}
