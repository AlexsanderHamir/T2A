package postgres

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLogStartupDBConfig_emitsExpectedFields(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))
	db, err := gorm.Open(sqlite.Open(":memory:"), nil)
	if err != nil {
		t.Fatal(err)
	}
	LogStartupDBConfig(lg, "taskapi", db)
	out := buf.String()
	if !strings.Contains(out, `"operation":"taskapi.db_config"`) {
		t.Fatalf("missing operation: %q", out)
	}
	if !strings.Contains(out, `"max_open_conns":25`) {
		t.Fatalf("missing max_open_conns: %q", out)
	}
	if !strings.Contains(out, `"gorm_slow_query_ms":200`) {
		t.Fatalf("missing gorm_slow_query_ms: %q", out)
	}
}

func TestLogStartupDBConfig_operationMatchesCmd(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))
	db, err := gorm.Open(sqlite.Open(":memory:"), nil)
	if err != nil {
		t.Fatal(err)
	}
	LogStartupDBConfig(lg, "dbcheck", db)
	if !strings.Contains(buf.String(), `"operation":"dbcheck.db_config"`) {
		t.Fatalf("missing dbcheck operation: %q", buf.String())
	}
}

func TestLogStartupDBConfig_nilDBIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))
	LogStartupDBConfig(lg, "taskapi", nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no log, got %q", buf.String())
	}
}
