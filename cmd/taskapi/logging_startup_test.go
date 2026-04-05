package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestEmitTaskAPIFileLoggingConfig_emitsAtMinLevel(t *testing.T) {
	for _, lv := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		var buf bytes.Buffer
		prev := slog.Default()
		t.Cleanup(func() { slog.SetDefault(prev) })
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: lv})))
		emitTaskAPIFileLoggingConfig(lv)
		line := strings.TrimSpace(buf.String())
		if line == "" {
			t.Fatalf("level %s: expected one log line", lv)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("level %s: %v", lv, err)
		}
		if m["msg"] != "logging config" {
			t.Fatalf("level %s: msg %v", lv, m["msg"])
		}
		if m["operation"] != "taskapi.logging" {
			t.Fatalf("level %s: operation %v", lv, m["operation"])
		}
		if m["min_level"] != lv.String() {
			t.Fatalf("level %s: min_level field %v", lv, m["min_level"])
		}
		if m["json_file"] != true {
			t.Fatalf("level %s: json_file %v", lv, m["json_file"])
		}
	}
}
