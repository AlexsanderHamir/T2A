package postgres

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestConfigWithSlogLogger_emitsTraceJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := ConfigWithSlogLogger(lg)
	if cfg == nil || cfg.Logger == nil {
		t.Fatal("expected non-nil config and logger")
	}
	begin := time.Now().Add(-2 * time.Millisecond)
	cfg.Logger.Trace(context.Background(), begin, func() (string, int64) {
		return `SELECT 1 WHERE id = $1`, 1
	}, nil)
	out := buf.String()
	if !strings.Contains(out, `"msg":"SQL executed"`) {
		t.Fatalf("expected SQL executed in log, got %q", out)
	}
	if !strings.Contains(out, `"sql"`) {
		t.Fatalf("expected sql field in log, got %q", out)
	}
}
