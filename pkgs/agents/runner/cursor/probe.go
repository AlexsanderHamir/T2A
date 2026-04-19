package cursor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// DefaultProbeTimeout is the wall-clock cap for one Probe call.
// 5 seconds matches the Stage 4 plan's "fail loudly at startup" budget:
// long enough for a cold cursor binary to print its version on
// resource-constrained CI hosts, short enough that an unresponsive
// binary does not stall taskapi startup.
const DefaultProbeTimeout = 5 * time.Second

// ProbeFn matches the bits of os/exec the Probe needs. Tests inject a
// fake to avoid spawning a real binary; production wiring uses
// DefaultProbeFn.
type ProbeFn func(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// Probe runs `<binaryPath> --version` with a bounded deadline and
// returns the trimmed first line of stdout (or stderr, whichever is
// non-empty) as the version string. The agent worker supervisor calls
// this whenever app_settings.worker_enabled flips on (and on every
// /settings probe-cursor request) and uses the returned string as
// runner.Version() so the audit trail records the exact CLI build
// that produced each cycle.
//
// timeout <= 0 falls back to DefaultProbeTimeout. probe == nil falls
// back to DefaultProbeFn.
//
// Errors are wrapped with the binary path so the operator sees which
// invocation failed; non-zero exit codes are treated as a probe
// failure (the binary either does not understand --version or refused
// to run for a config reason — either is fail-fast worthy).
func Probe(ctx context.Context, binaryPath string, timeout time.Duration, probe ProbeFn) (string, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Probe",
		"binary", binaryPath, "timeout_ns", int64(timeout))
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return "", errors.New("cursor probe: empty binary path")
	}
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	if probe == nil {
		probe = DefaultProbeFn
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout, stderr, exitCode, err := probe(probeCtx, binaryPath, "--version")
	if err != nil {
		if probeCtx.Err() != nil {
			return "", fmt.Errorf("cursor probe %q: timed out after %s: %w", binaryPath, timeout, err)
		}
		return "", fmt.Errorf("cursor probe %q: exec failed: %w", binaryPath, err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("cursor probe %q: exit %d (stderr=%q)", binaryPath, exitCode, trimForLog(stderr))
	}

	version := firstNonEmptyLine(stdout)
	if version == "" {
		version = firstNonEmptyLine(stderr)
	}
	if version == "" {
		return "", fmt.Errorf("cursor probe %q: empty --version output", binaryPath)
	}
	return version, nil
}

// DefaultProbeFn is the production ProbeFn used when callers do not
// inject a fake. It mirrors the shape of defaultExecFn but does not
// scrub env or stream stdin — `--version` invocations need neither.
func DefaultProbeFn(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.DefaultProbeFn",
		"name", name, "argc", len(args))
	cmd := exec.CommandContext(ctx, name, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode, err
}

// ResolveBinaryPath returns the absolute path that exec.Command would use
// for binaryPath (via PATH lookup), or the trimmed input when LookPath
// fails. Designed so callers can show the operator the concrete path that
// got executed — particularly when the input is a bare command name like
// "cursor-agent" and the actual binary lives somewhere on PATH that the
// operator never typed. Returns "" for empty/whitespace input so callers
// can distinguish "nothing configured" from "PATH lookup miss".
//
// Best-effort: a LookPath failure is not propagated because the caller
// (e.g. registry.Probe) still wants to attempt the probe with the bare
// name and surface the real exec error rather than a preflight LookPath
// error that would be redundant.
func ResolveBinaryPath(binaryPath string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.ResolveBinaryPath",
		"binary", binaryPath)
	p := strings.TrimSpace(binaryPath)
	if p == "" {
		return ""
	}
	if abs, err := exec.LookPath(p); err == nil {
		return abs
	}
	return p
}

// firstNonEmptyLine returns the first non-empty trimmed line of b, or
// "" when b is empty / whitespace-only.
func firstNonEmptyLine(b []byte) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.firstNonEmptyLine",
		"bytes", len(b))
	for _, line := range strings.Split(string(b), "\n") {
		if v := strings.TrimSpace(line); v != "" {
			return v
		}
	}
	return ""
}

// trimForLog truncates b for inclusion in error messages so a chatty
// stderr does not blow up the probe error string.
func trimForLog(b []byte) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.trimForLog",
		"bytes", len(b))
	const max = 256
	s := strings.TrimSpace(string(b))
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
