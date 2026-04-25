package cursor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

func trimLeadingPartialRune(b []byte) []byte {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "agents.runner.cursor.trimLeadingPartialRune", "bytes", len(b))
	for len(b) > 0 && !utf8.RuneStart(b[0]) {
		b = b[1:]
	}
	return b
}

var envDeniedKeys = map[string]struct{}{
	"DATABASE_URL": {},
}

var defaultPassthroughEnvKeys = []string{
	"PATH",
	"HOME",
	"USERPROFILE",
	"SYSTEMDRIVE",
	"SYSTEMROOT",
	"WINDIR",
	"COMSPEC",
	"PATHEXT",
	"LOCALAPPDATA",
	"APPDATA",
	"PROGRAMDATA",
	"ALLUSERSPROFILE",
	"PUBLIC",
	"TEMP",
	"TMP",
	"PROGRAMFILES",
	"PROGRAMFILES(X86)",
	"PROGRAMW6432",
	"COMMONPROGRAMFILES",
	"COMMONPROGRAMFILES(X86)",
	"USERNAME",
	"USERDOMAIN",
	"COMPUTERNAME",
	"LOGONSERVER",
	"SESSIONNAME",
	"OS",
	"PROCESSOR_ARCHITECTURE",
	"PROCESSOR_IDENTIFIER",
	"PROCESSOR_LEVEL",
	"PROCESSOR_REVISION",
	"NUMBER_OF_PROCESSORS",
}

func buildEnv(reqEnv map[string]string, extraKeys []string) []string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.buildEnv",
		"req_env_count", len(reqEnv), "extra_keys", len(extraKeys))
	allowed := make(map[string]struct{}, len(defaultPassthroughEnvKeys)+len(extraKeys))
	for _, k := range defaultPassthroughEnvKeys {
		allowed[k] = struct{}{}
	}
	for _, k := range extraKeys {
		if k == "" || isDeniedEnvKey(k) {
			continue
		}
		allowed[k] = struct{}{}
	}
	merged := map[string]string{}
	for k := range allowed {
		if v := os.Getenv(k); v != "" {
			merged[k] = v
		}
	}
	for k, v := range reqEnv {
		if k == "" || isDeniedEnvKey(k) {
			continue
		}
		merged[k] = v
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

func isDeniedEnvKey(k string) bool {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.isDeniedEnvKey", "key", k)
	if _, denied := envDeniedKeys[k]; denied {
		return true
	}
	return strings.HasPrefix(k, "T2A_")
}

func liveHomePaths() []string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.liveHomePaths")
	out := make([]string, 0, 2)
	for _, k := range []string{"HOME", "USERPROFILE"} {
		if v := os.Getenv(k); v != "" {
			out = append(out, v)
		}
	}
	return out
}

var (
	authHeaderRe   = regexp.MustCompile(`(?i)(authorization:[ \t]*)([^\r\n]+)`)
	cookieHeaderRe = regexp.MustCompile(`(?i)\b((?:set-)?cookie:[ \t]*)([^\r\n]+)`)
	t2aEnvRe       = regexp.MustCompile(`(T2A_[A-Z0-9_]+)\s*=\s*\S+`)
)

// Redact replaces secret-shaped substrings in s with [REDACTED] markers
// and rewrites absolute home paths to "~". It is exported so callers
// (worker logs, future adapters) can apply the same floor.
func Redact(s string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Redact", "bytes", len(s))
	return redact(s, liveHomePaths())
}

func redact(s string, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.redact",
		"bytes", len(s), "home_paths", len(homePaths))
	if s == "" {
		return s
	}
	out := authHeaderRe.ReplaceAllString(s, "${1}[REDACTED]")
	out = cookieHeaderRe.ReplaceAllString(out, "${1}[REDACTED]")
	out = t2aEnvRe.ReplaceAllString(out, "$1=[REDACTED]")
	for _, hp := range homePaths {
		if hp == "" {
			continue
		}
		out = strings.ReplaceAll(out, hp, "~")
	}
	return out
}

func withOptionalTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.withOptionalTimeout",
		"timeout_ns", int64(d))
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

func isCtxErr(ctx context.Context) bool {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.isCtxErr")
	err := ctx.Err()
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func defaultExecFn(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) ([]byte, []byte, int, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.defaultExecFn",
		"dir", dir, "env_count", len(env), "stdin_bytes", len(stdin), "name", name, "argc", len(args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
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

func defaultStreamExecFn(ctx context.Context, dir string, env []string, stdin []byte, name string, onStdoutLine func([]byte), args ...string) ([]byte, []byte, int, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.defaultStreamExecFn",
		"dir", dir, "env_count", len(env), "stdin_bytes", len(stdin), "name", name, "argc", len(args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, 0, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, 0, err
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutDone := make(chan error, 1)
	stderrDone := make(chan error, 1)
	go func() {
		stdoutDone <- scanStdoutLines(stdoutPipe, &stdoutBuf, onStdoutLine)
	}()
	go func() {
		_, err := io.Copy(&stderrBuf, stderrPipe)
		stderrDone <- err
	}()

	waitErr := cmd.Wait()
	stdoutErr := <-stdoutDone
	stderrErr := <-stderrDone
	if waitErr == nil {
		stdoutErr = normalizePipeReadError(stdoutErr)
		stderrErr = normalizePipeReadError(stderrErr)
		if stdoutErr != nil {
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), 0, stdoutErr
		}
		if stderrErr != nil {
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), 0, stderrErr
		}
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), 0, nil
	}
	if ctx.Err() != nil {
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), 0, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitErr.ExitCode(), nil
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), 0, waitErr
}

func scanStdoutLines(r io.Reader, dst *bytes.Buffer, onLine func([]byte)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		dst.Write(line)
		dst.WriteByte('\n')
		if onLine != nil {
			onLine(line)
		}
	}
	return scanner.Err()
}

func normalizePipeReadError(err error) error {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.normalizePipeReadError",
		"err", err)
	if isClosedPipeReadError(err) {
		return nil
	}
	return err
}

func isClosedPipeReadError(err error) bool {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.isClosedPipeReadError",
		"err", err)
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "file already closed")
}
