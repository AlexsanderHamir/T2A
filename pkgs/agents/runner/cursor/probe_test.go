package cursor_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
)

func TestProbe_HappyPath_returnsTrimmedFirstLine(t *testing.T) {
	t.Parallel()
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		if name != "/usr/local/bin/cursor" {
			t.Fatalf("binary = %q", name)
		}
		if len(args) != 1 || args[0] != "--version" {
			t.Fatalf("args = %v", args)
		}
		return []byte("  cursor-cli 1.2.3\nbuild abc\n"), nil, 0, nil
	}
	got, err := cursor.Probe(context.Background(), "/usr/local/bin/cursor", time.Second, fake)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got != "cursor-cli 1.2.3" {
		t.Fatalf("version = %q, want %q", got, "cursor-cli 1.2.3")
	}
}

func TestProbe_FallsBackToStderrWhenStdoutEmpty(t *testing.T) {
	t.Parallel()
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, []byte("cursor 0.99-rc1\n"), 0, nil
	}
	got, err := cursor.Probe(context.Background(), "cursor", time.Second, fake)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got != "cursor 0.99-rc1" {
		t.Fatalf("version = %q", got)
	}
}

func TestProbe_NonZeroExit_isError(t *testing.T) {
	t.Parallel()
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, []byte("cursor: invalid flag\n"), 2, nil
	}
	_, err := cursor.Probe(context.Background(), "cursor", time.Second, fake)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exit 2") {
		t.Fatalf("err = %v", err)
	}
}

func TestProbe_ExecError_isWrapped(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("file not found")
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, nil, 0, wantErr
	}
	_, err := cursor.Probe(context.Background(), "cursor", time.Second, fake)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("err chain missing wantErr: %v", err)
	}
}

func TestProbe_Timeout_returnsTimeoutError(t *testing.T) {
	t.Parallel()
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		<-ctx.Done()
		return nil, nil, 0, ctx.Err()
	}
	_, err := cursor.Probe(context.Background(), "cursor", 20*time.Millisecond, fake)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v", err)
	}
}

func TestProbe_EmptyOutput_isError(t *testing.T) {
	t.Parallel()
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("   \n\n"), []byte(""), 0, nil
	}
	_, err := cursor.Probe(context.Background(), "cursor", time.Second, fake)
	if err == nil {
		t.Fatal("expected error for empty version output")
	}
}

func TestProbe_EmptyBinaryPath_isError(t *testing.T) {
	t.Parallel()
	_, err := cursor.Probe(context.Background(), "  ", time.Second, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProbe_DefaultsApplied(t *testing.T) {
	t.Parallel()
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Fatal("ctx has no deadline")
		}
		if time.Until(dl) > cursor.DefaultProbeTimeout+time.Second {
			t.Fatalf("deadline farther than default: %s", time.Until(dl))
		}
		return []byte("v1\n"), nil, 0, nil
	}
	if _, err := cursor.Probe(context.Background(), "cursor", 0, fake); err != nil {
		t.Fatalf("probe: %v", err)
	}
}
