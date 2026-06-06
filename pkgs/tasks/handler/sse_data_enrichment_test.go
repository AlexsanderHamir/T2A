package handler

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSSEHub_Publish_carries_data_payload(t *testing.T) {
	h := NewSSEHub()
	ch, cancel := h.Subscribe()
	defer cancel()

	type fakeTaskTree struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	tree := fakeTaskTree{ID: "abc", Title: "renamed"}

	h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "abc", Data: tree})

	select {
	case line := <-ch:
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Fatalf("unmarshal: %v line=%q", err, line)
		}
		if _, ok := raw["data"]; !ok {
			t.Fatalf("payload missing data field: %s", line)
		}
		var got fakeTaskTree
		if err := json.Unmarshal(raw["data"], &got); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		if got.ID != "abc" || got.Title != "renamed" {
			t.Errorf("data = %+v, want {abc, renamed}", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for enriched event")
	}
}

func TestSSEHub_DataBearingEvents_are_not_coalesced(t *testing.T) {
	// Even with coalesce enabled, two data-bearing events for the same
	// {type, id} must BOTH fanout — discarding the second would drop
	// the newer entity payload, defeating the whole point of enrichment.
	opts := DefaultSSEHubOptions()
	opts.CoalesceWindow = 50 * time.Millisecond
	h := NewSSEHubWith(opts)
	ch, cancel := h.Subscribe()
	defer cancel()

	h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "abc", Data: map[string]string{"v": "1"}})
	h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "abc", Data: map[string]string{"v": "2"}})

	received := 0
	deadline := time.After(500 * time.Millisecond)
	for received < 2 {
		select {
		case <-ch:
			received++
		case <-deadline:
			t.Fatalf("only received %d frames, want 2 (data-bearing events must not be coalesced)", received)
		}
	}
}

func TestSSEHub_HintEvents_still_coalesce_within_window(t *testing.T) {
	// Regression guard: removing coalescing for data-bearing events must
	// NOT remove it for hint-only frames. Two hint-only task_updated
	// frames within the window still collapse to one.
	opts := DefaultSSEHubOptions()
	opts.CoalesceWindow = 200 * time.Millisecond
	h := NewSSEHubWith(opts)
	ch, cancel := h.Subscribe()
	defer cancel()

	h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "abc"})
	h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "abc"})

	received := 0
	deadline := time.After(150 * time.Millisecond)
loop:
	for {
		select {
		case <-ch:
			received++
		case <-deadline:
			break loop
		}
	}
	if received != 1 {
		t.Errorf("hint-only coalesce delivered %d frames, want 1", received)
	}
}
