package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSSE_subscriberGaugeTracksSubscribe(t *testing.T) {
	g := middleware.SSESubscribersGauge()
	base := testutil.ToFloat64(g)
	h := NewSSEHub()
	_, cancel := h.Subscribe()
	if got := testutil.ToFloat64(g); got != base+1 {
		t.Fatalf("after subscribe: got %v want %v", got, base+1)
	}
	cancel()
	if got := testutil.ToFloat64(g); got != base {
		t.Fatalf("after cancel: got %v want %v", got, base)
	}
}

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

// TestSSEHub_Publish_recordsDroppedFramesCounter pins the new
// taskapi_sse_dropped_frames_total observable wired in this session: when a
// subscriber's bounded channel fills up (default cap 32 inside Subscribe),
// further Publish calls drop on the `default` branch instead of blocking the
// publisher. Before this counter, those drops were silent — a stuck client
// would only surface as missing UI updates with no metric trail.
//
// Setup: one slow subscriber that never reads, then 32 publishes to fill the
// channel (each one delivered, none dropped) followed by 5 more publishes
// (all dropped because the channel is full). Asserts the counter advanced by
// exactly 5 between snapshots — proves both the per-fanout count is correct
// and that the dropped-loop iteration matches the subscriber count (1 here).
//
// A second `idleSecondSubscriber` is added partway to confirm the counter
// counts per-frame-per-subscriber, not per-fanout: 3 publishes after the
// second subscriber joins, each of which drops on BOTH subscribers (since
// neither is reading), should advance the counter by 6 (3 publishes ×
// 2 subscribers). This pins the helper signature (`RecordSSEDroppedFrames(n)`
// gets the full per-fanout sum) so a future refactor that called the helper
// once per fanout instead of once per dropped subscriber would fail loudly.
func TestSSEHub_Publish_recordsDroppedFramesCounter(t *testing.T) {
	c := middleware.SSEDroppedFramesCounter()
	base := testutil.ToFloat64(c)

	h := NewSSEHub()
	_, cancel := h.Subscribe()
	defer cancel()

	const subscriberBufferCap = 32
	for i := 0; i < subscriberBufferCap; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "fill"})
	}
	if got := testutil.ToFloat64(c); got != base {
		t.Fatalf("counter advanced before drops: got %v want %v", got, base)
	}

	const overflowOneSub = 5
	for i := 0; i < overflowOneSub; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "drop1"})
	}
	if got, want := testutil.ToFloat64(c), base+overflowOneSub; got != want {
		t.Fatalf("after %d drops on 1 subscriber: counter=%v want %v", overflowOneSub, got, want)
	}

	_, cancel2 := h.Subscribe()
	defer cancel2()

	const dropFanoutFrames = 3
	for i := 0; i < dropFanoutFrames+subscriberBufferCap; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "drop2"})
	}
	wantAfter := base + overflowOneSub + dropFanoutFrames*2 + subscriberBufferCap*1
	if got := testutil.ToFloat64(c); got != wantAfter {
		t.Fatalf("after second-subscriber phase: counter=%v want %v (overflowOneSub=%d + dropFanoutFrames*2=%d + subscriberBufferCap*1=%d for the still-full first sub during the 32 fills of the second sub)",
			got, wantAfter, overflowOneSub, dropFanoutFrames*2, subscriberBufferCap)
	}
}

func TestHTTP_SSE_responseHeaders(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q want text/event-stream", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q want no-store (docs/API-SSE.md)", got)
	}
	if got := res.Header.Get("Connection"); got != "keep-alive" {
		t.Errorf("Connection = %q want keep-alive", got)
	}
	if got := res.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q want no", got)
	}
	_, _ = io.Copy(io.Discard, res.Body)
}

func TestHTTP_SSE_receivesEventAfterCreate(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
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
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"sse","priority":"medium"}`))
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
