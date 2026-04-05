package postgres

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestSlowQueryThresholdForGORM(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"default", "", 200 * time.Millisecond},
		{"zero_disables", "0", 0},
		{"custom_ms", "500", 500 * time.Millisecond},
		{"invalid_falls_back", "nope", 200 * time.Millisecond},
		{"negative_falls_back", "-3", 200 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("T2A_GORM_SLOW_QUERY_MS", tc.env)
			if got := slowQueryThresholdForGORM(); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestSlowQueryThresholdMS(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{"default", "", 200},
		{"zero_disables", "0", 0},
		{"custom_ms", "500", 500},
		{"invalid_falls_back", "nope", 200},
		{"negative_falls_back", "-3", 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("T2A_GORM_SLOW_QUERY_MS", tc.env)
			if got := SlowQueryThresholdMS(); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestNewSlogLogger_slowSQLLogsAtWarn(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	// Negative threshold: GORM treats any positive elapsed as "slow" (elapsed > negative threshold).
	gl := gormlogger.NewSlogLogger(lg, gormlogger.Config{
		LogLevel:                  gormlogger.Info,
		SlowThreshold:             -1,
		ParameterizedQueries:      true,
		IgnoreRecordNotFoundError: true,
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gl})
	if err != nil {
		t.Fatal(err)
	}
	var one int
	if err := db.Raw("SELECT 1").Scan(&one).Error; err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "SQL executed") {
		t.Fatalf("expected SQL in log, got %q", out)
	}
	if !strings.Contains(strings.ToUpper(out), "WARN") {
		t.Fatalf("expected WARN level for slow SQL, got %q", out)
	}
}

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
