package logctx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWrapSlogHandlerWithRequestContext_addsRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h)
	ctx := ContextWithRequestID(context.Background(), "corr-test-1")
	lg.Log(ctx, slog.LevelInfo, "probe", "k", "v")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if got := line["request_id"]; got != "corr-test-1" {
		t.Fatalf("request_id %v", got)
	}
}

func TestWrapSlogHandlerWithRequestContext_noIDWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	h := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h)
	lg.Log(context.Background(), slog.LevelInfo, "probe")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if _, ok := line["request_id"]; ok {
		t.Fatal("unexpected request_id")
	}
}

func TestRequestIDFromContext_empty(t *testing.T) {
	if RequestIDFromContext(nil) != "" {
		t.Fatal("nil ctx")
	}
	if RequestIDFromContext(context.Background()) != "" {
		t.Fatal("background")
	}
}

func TestContextWithRequestID_emptyLeavesContextUnchanged(t *testing.T) {
	base := context.Background()
	if got := ContextWithRequestID(base, ""); got != base {
		t.Fatal("empty id should return ctx unchanged")
	}
}

func TestWrapSlogHandlerWithRequestContext_nilReturnsNil(t *testing.T) {
	if WrapSlogHandlerWithRequestContext(nil) != nil {
		t.Fatal("want nil")
	}
}

func TestWrapSlogHandlerWithRequestContext_WithAttrsStillAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h).With("scope", "obs-test")
	ctx := ContextWithRequestID(context.Background(), "rid-attrs")
	lg.Log(ctx, slog.LevelInfo, "msg")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line["request_id"] != "rid-attrs" {
		t.Fatalf("request_id %v", line["request_id"])
	}
	if line["scope"] != "obs-test" {
		t.Fatalf("scope %v", line["scope"])
	}
}

func TestWrapSlogHandlerWithRequestContext_WithGroupStillAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h).WithGroup("g").With("k", "v")
	ctx := ContextWithRequestID(context.Background(), "rid-grp")
	lg.Log(ctx, slog.LevelInfo, "inside")

	if !strings.Contains(buf.String(), `"request_id":"rid-grp"`) {
		t.Fatalf("missing request_id in %q", buf.String())
	}
}

func TestWrapSlogHandlerWithLogSequence_nilReturnsNil(t *testing.T) {
	var p atomic.Uint64
	if WrapSlogHandlerWithLogSequence(nil, &p) != nil {
		t.Fatal("want nil")
	}
}

func TestWrapSlogHandlerWithLogSequence_requestScopeMonotonic(t *testing.T) {
	var buf bytes.Buffer
	var processSeq atomic.Uint64
	base := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h := WrapSlogHandlerWithLogSequence(base, &processSeq)
	lg := slog.New(h)
	ctx := ContextWithRequestID(ContextWithLogSeq(context.Background()), "seq-rid")
	lg.Log(ctx, slog.LevelInfo, "a")
	lg.Log(ctx, slog.LevelInfo, "b")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines: %d %q", len(lines), buf.String())
	}
	var first, second map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &first)
	_ = json.Unmarshal([]byte(lines[1]), &second)
	if int(first["log_seq"].(float64)) != 1 || int(second["log_seq"].(float64)) != 2 {
		t.Fatalf("log_seq: %v %v", first["log_seq"], second["log_seq"])
	}
	if first["log_seq_scope"] != "request" || second["log_seq_scope"] != "request" {
		t.Fatalf("scope: %v %v", first["log_seq_scope"], second["log_seq_scope"])
	}
}

func TestWrapSlogHandlerWithLogSequence_processFallbackWhenNoRequestCounter(t *testing.T) {
	var buf bytes.Buffer
	var processSeq atomic.Uint64
	h := WrapSlogHandlerWithLogSequence(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}), &processSeq)
	slog.New(h).Log(context.Background(), slog.LevelInfo, "startup")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if int(line["log_seq"].(float64)) != 1 {
		t.Fatalf("log_seq: %v", line["log_seq"])
	}
	if line["log_seq_scope"] != "process" {
		t.Fatalf("log_seq_scope: %v", line["log_seq_scope"])
	}
}

func TestWrapSlogHandlerWithLogSequence_outerSeqInnerRequestIDMatchesTaskapi(t *testing.T) {
	var buf bytes.Buffer
	var processSeq atomic.Uint64
	jsonH := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	inner := WrapSlogHandlerWithRequestContext(jsonH)
	outer := WrapSlogHandlerWithLogSequence(inner, &processSeq)
	lg := slog.New(outer)
	ctx := ContextWithRequestID(ContextWithLogSeq(context.Background()), "wrap-rid")
	lg.Log(ctx, slog.LevelInfo, "probe")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line["request_id"] != "wrap-rid" {
		t.Fatalf("request_id: %v", line["request_id"])
	}
	if line["log_seq_scope"] != "request" {
		t.Fatalf("log_seq_scope: %v", line["log_seq_scope"])
	}
}
