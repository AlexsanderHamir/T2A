package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

func TestRunObserved_successEmitsHelperInThenOut(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))

	ctx := logctx.ContextWithRequestID(logctx.ContextWithLogSeq(context.Background()), "obs-rid")
	err := RunObserved(ctx, "unitObserved", []any{"in_k", "in_v"}, func(ctx context.Context) ([]any, error) {
		return []any{"out_k", 42}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	helperLines := helperIOLinesFromBuffer(buf.String())
	if len(helperLines) != 2 {
		t.Fatalf("want 2 helper.io lines, got %d: %q", len(helperLines), buf.String())
	}
	var inLine, outLine map[string]any
	if err := json.Unmarshal([]byte(helperLines[0]), &inLine); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(helperLines[1]), &outLine); err != nil {
		t.Fatal(err)
	}
	if inLine["msg"] != "helper.io" || outLine["msg"] != "helper.io" {
		t.Fatalf("msg: %v %v", inLine["msg"], outLine["msg"])
	}
	if inLine["phase"] != "helper_in" || outLine["phase"] != "helper_out" {
		t.Fatalf("phase: %v %v", inLine["phase"], outLine["phase"])
	}
	if inLine["function"] != "unitObserved" || outLine["function"] != "unitObserved" {
		t.Fatalf("function: %v %v", inLine["function"], outLine["function"])
	}
	if inLine["obs_category"] != "helper_io" || outLine["obs_category"] != "helper_io" {
		t.Fatalf("obs_category: %v %v", inLine["obs_category"], outLine["obs_category"])
	}
	if inLine["in_k"] != "in_v" {
		t.Fatalf("in kv: %v", inLine["in_k"])
	}
	if int(outLine["out_k"].(float64)) != 42 {
		t.Fatalf("out kv: %v", outLine["out_k"])
	}
	if int(inLine["log_seq"].(float64)) != 1 || int(outLine["log_seq"].(float64)) != 2 {
		t.Fatalf("log_seq order: %v %v", inLine["log_seq"], outLine["log_seq"])
	}
}

func TestRunObserved_errorAppendsErrOnOut(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))

	ctx := logctx.ContextWithRequestID(logctx.ContextWithLogSeq(context.Background()), "obs-err")
	sentinel := errors.New("boom")
	err := RunObserved(ctx, "unitFail", nil, func(ctx context.Context) ([]any, error) {
		return nil, sentinel
	})
	if err != sentinel {
		t.Fatalf("err: %v", err)
	}

	helperLines := helperIOLinesFromBuffer(buf.String())
	if len(helperLines) != 2 {
		t.Fatalf("helper.io lines: %d", len(helperLines))
	}
	var outLine map[string]any
	if err := json.Unmarshal([]byte(helperLines[1]), &outLine); err != nil {
		t.Fatal(err)
	}
	if outLine["phase"] != "helper_out" {
		t.Fatalf("phase: %v", outLine["phase"])
	}
	if !strings.Contains(outLine["err"].(string), "boom") {
		t.Fatalf("err field: %v", outLine["err"])
	}
}

func helperIOLinesFromBuffer(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if obj["msg"] == "helper.io" {
			out = append(out, line)
		}
	}
	return out
}
