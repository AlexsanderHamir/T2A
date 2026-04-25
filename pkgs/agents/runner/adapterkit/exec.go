package adapterkit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	// DefaultScannerInitialBufferBytes is the initial stdout line scanner
	// buffer used by DefaultStreamExec.
	DefaultScannerInitialBufferBytes = 64 * 1024
	// DefaultScannerMaxBufferBytes is the largest single stdout line accepted
	// by DefaultStreamExec before bufio.Scanner reports ErrTooLong.
	DefaultScannerMaxBufferBytes = 1024 * 1024
)

// ExecFunc shells out to a command and returns captured stdout/stderr, process
// exit code, and start/wait errors. A non-zero exit code with nil error means
// the process completed and exited unsuccessfully.
type ExecFunc func(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// StreamExecFunc is the streaming form of ExecFunc. It calls onStdoutLine for
// each complete stdout line while still returning the fully captured streams.
type StreamExecFunc func(ctx context.Context, dir string, env []string, stdin []byte, name string, onStdoutLine func([]byte), args ...string) (stdout []byte, stderr []byte, exitCode int, err error)

// DefaultExec is the production ExecFunc implementation backed by os/exec.
func DefaultExec(ctx context.Context, dir string, env []string, stdin []byte, name string, args ...string) ([]byte, []byte, int, error) {
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

// DefaultStreamExec is the production StreamExecFunc implementation backed by
// os/exec and stdout/stderr pipes.
func DefaultStreamExec(ctx context.Context, dir string, env []string, stdin []byte, name string, onStdoutLine func([]byte), args ...string) ([]byte, []byte, int, error) {
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
		stdoutDone <- ScanStdoutLines(stdoutPipe, &stdoutBuf, onStdoutLine)
	}()
	go func() {
		_, err := io.Copy(&stderrBuf, stderrPipe)
		stderrDone <- err
	}()

	waitErr := cmd.Wait()
	stdoutErr := <-stdoutDone
	stderrErr := <-stderrDone
	if waitErr == nil {
		stdoutErr = NormalizePipeReadError(stdoutErr)
		stderrErr = NormalizePipeReadError(stderrErr)
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

// ScanStdoutLines copies complete lines from r into dst and invokes onLine
// with a stable copy of each line.
func ScanStdoutLines(r io.Reader, dst *bytes.Buffer, onLine func([]byte)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, DefaultScannerInitialBufferBytes), DefaultScannerMaxBufferBytes)
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

// NormalizePipeReadError maps benign closed-pipe read errors to nil.
func NormalizePipeReadError(err error) error {
	if IsClosedPipeReadError(err) {
		return nil
	}
	return err
}

// IsClosedPipeReadError reports whether err represents reading a pipe after
// the child process has already closed it.
func IsClosedPipeReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "file already closed")
}
