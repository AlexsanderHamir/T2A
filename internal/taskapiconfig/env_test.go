package taskapiconfig

import (
	"log/slog"
	"testing"
	"time"
)

func TestUserTaskAgentQueueCap(t *testing.T) {
	t.Setenv(EnvUserTaskAgentQueueCap, "")
	if UserTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("unset got %d want %d", UserTaskAgentQueueCap(), defaultUserTaskAgentQueueCap)
	}
	t.Setenv(EnvUserTaskAgentQueueCap, "10")
	if UserTaskAgentQueueCap() != 10 {
		t.Fatalf("got %d", UserTaskAgentQueueCap())
	}
	t.Setenv(EnvUserTaskAgentQueueCap, "0")
	if UserTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("zero should fall back to default, got %d", UserTaskAgentQueueCap())
	}
	t.Setenv(EnvUserTaskAgentQueueCap, "-1")
	if UserTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("negative should fall back to default, got %d", UserTaskAgentQueueCap())
	}
	t.Setenv(EnvUserTaskAgentQueueCap, "nope")
	if UserTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("invalid should fall back to default, got %d", UserTaskAgentQueueCap())
	}
}

func TestEnvTruthy(t *testing.T) {
	t.Setenv("T2A_ENV_TRUTHY_TEST", "")
	if EnvTruthy("T2A_ENV_TRUTHY_TEST") {
		t.Fatal("empty should be false")
	}
	for _, v := range []string{"1", "true", "TRUE", "yes", "Yes", "on", "ON"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("T2A_ENV_TRUTHY_TEST", v)
			if !EnvTruthy("T2A_ENV_TRUTHY_TEST") {
				t.Fatalf("want true for %q", v)
			}
		})
	}
	t.Setenv("T2A_ENV_TRUTHY_TEST", "0")
	if EnvTruthy("T2A_ENV_TRUTHY_TEST") {
		t.Fatal("0 should be false")
	}
}

func TestLoggingMinimized_flagWins(t *testing.T) {
	t.Setenv(EnvDisableLogging, "")
	if !LoggingMinimized(true) {
		t.Fatal("flag true should minimize")
	}
}

func TestLoggingMinimized_env(t *testing.T) {
	t.Setenv(EnvDisableLogging, "1")
	if !LoggingMinimized(false) {
		t.Fatal("env should minimize")
	}
	t.Setenv(EnvDisableLogging, "")
	if LoggingMinimized(false) {
		t.Fatal("unset env should not minimize")
	}
}

func TestResolveLogLevel_defaultsToInfo(t *testing.T) {
	t.Setenv(EnvLogLevel, "")
	got, err := ResolveLogLevel("")
	if err != nil {
		t.Fatal(err)
	}
	if got != slog.LevelInfo {
		t.Fatalf("default: got %v want %v", got, slog.LevelInfo)
	}
}

func TestResolveLogLevel_envWhenFlagEmpty(t *testing.T) {
	t.Setenv(EnvLogLevel, "info")
	got, err := ResolveLogLevel("")
	if err != nil {
		t.Fatal(err)
	}
	if got != slog.LevelInfo {
		t.Fatalf("got %v want info", got)
	}
}

func TestResolveLogLevel_flagOverridesEnv(t *testing.T) {
	t.Setenv(EnvLogLevel, "info")
	got, err := ResolveLogLevel("error")
	if err != nil {
		t.Fatal(err)
	}
	if got != slog.LevelError {
		t.Fatalf("got %v want error", got)
	}
}

func TestResolveLogLevel_caseInsensitiveAliases(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"Info", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	} {
		got, err := ResolveLogLevel(tt.in)
		if err != nil {
			t.Fatalf("%q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestResolveLogLevel_invalid(t *testing.T) {
	t.Setenv(EnvLogLevel, "")
	_, err := ResolveLogLevel("verbose")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveLogLevel_invalidEnv(t *testing.T) {
	t.Setenv(EnvLogLevel, "nope")
	_, err := ResolveLogLevel("")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSETestTickerInterval(t *testing.T) {
	t.Run("defaults to 3s when unset", func(t *testing.T) {
		t.Setenv(EnvSSETestInterval, "")
		if got := SSETestTickerInterval(); got != defaultSSETestInterval {
			t.Fatalf("got %v want %v", got, defaultSSETestInterval)
		}
	})
	t.Run("zero disables ticker", func(t *testing.T) {
		t.Setenv(EnvSSETestInterval, "0")
		if got := SSETestTickerInterval(); got != 0 {
			t.Fatalf("got %v want 0", got)
		}
	})
	t.Run("custom duration", func(t *testing.T) {
		t.Setenv(EnvSSETestInterval, "7s")
		if got := SSETestTickerInterval(); got != 7*time.Second {
			t.Fatalf("got %v", got)
		}
	})
}

func TestListenHost(t *testing.T) {
	t.Run("defaults to localhost when flag and env are empty", func(t *testing.T) {
		t.Setenv(EnvListenHost, "")
		if got := ListenHost(""); got != "127.0.0.1" {
			t.Fatalf("got %q want 127.0.0.1", got)
		}
	})
	t.Run("uses env when flag is empty", func(t *testing.T) {
		t.Setenv(EnvListenHost, "0.0.0.0")
		if got := ListenHost(""); got != "0.0.0.0" {
			t.Fatalf("got %q want 0.0.0.0", got)
		}
	})
	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(EnvListenHost, "0.0.0.0")
		if got := ListenHost("127.0.0.1"); got != "127.0.0.1" {
			t.Fatalf("got %q want 127.0.0.1", got)
		}
	})
}
