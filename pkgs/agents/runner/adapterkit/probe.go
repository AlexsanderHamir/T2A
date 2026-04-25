package adapterkit

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const DefaultProbeLogBytes = 256

// ProbeFunc is the small command execution surface needed by bounded probes.
type ProbeFunc func(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// DefaultProbeFunc runs a short command and captures stdout/stderr.
func DefaultProbeFunc(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
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

// RunProbe runs probe with a timeout. timeout <= 0 means no additional
// deadline is applied.
func RunProbe(ctx context.Context, timeout time.Duration, probe ProbeFunc, name string, args ...string) ([]byte, []byte, int, error) {
	if probe == nil {
		probe = DefaultProbeFunc
	}
	if timeout <= 0 {
		return probe(ctx, name, args...)
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return probe(probeCtx, name, args...)
}

// ResolveBinaryPath returns the path exec.Command would use after PATH lookup,
// or the trimmed input when lookup fails.
func ResolveBinaryPath(binaryPath string) string {
	p := strings.TrimSpace(binaryPath)
	if p == "" {
		return ""
	}
	if abs, err := exec.LookPath(p); err == nil {
		return abs
	}
	return p
}

// FirstNonEmptyLine returns the first non-empty trimmed line of b.
func FirstNonEmptyLine(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		if v := strings.TrimSpace(line); v != "" {
			return v
		}
	}
	return ""
}

// TrimForLog trims a byte buffer for inclusion in an error string.
func TrimForLog(b []byte, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = DefaultProbeLogBytes
	}
	s := strings.TrimSpace(string(b))
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "…"
}
