package repo

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// LineCount returns the number of lines in a file (newline-separated).
func LineCount(absPath string) (int, error) {
	slog.Debug("trace", "operation", "repo.LineCount")
	fi, err := os.Stat(absPath)
	if err != nil {
		return 0, err
	}
	if fi.IsDir() {
		return 0, fmt.Errorf("is a directory")
	}
	if fi.Size() > maxFileReadBytes {
		return 0, fmt.Errorf("%w: file too large", domain.ErrInvalidInput)
	}
	f, err := os.Open(absPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	lr := io.LimitReader(f, maxFileReadBytes+1)
	buf := make([]byte, 32*1024)
	var n int
	var total int64
	var last byte
	hasData := false
	for {
		readN, readErr := lr.Read(buf)
		if readN > 0 {
			chunk := buf[:readN]
			total += int64(readN)
			n += bytes.Count(chunk, []byte{'\n'})
			last = chunk[readN-1]
			hasData = true
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, readErr
		}
	}
	if total > maxFileReadBytes {
		return 0, fmt.Errorf("%w: file too large", domain.ErrInvalidInput)
	}
	if !hasData {
		return 0, nil
	}
	if last != '\n' {
		n++
	}
	return n, nil
}

// ValidateRange returns nil if start..end are valid 1-based inclusive line numbers for the file.
func ValidateRange(absPath string, start, end int) error {
	slog.Debug("trace", "operation", "repo.ValidateRange")
	if err := validateRangeBounds(start, end); err != nil {
		return err
	}
	n, err := LineCount(absPath)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	return validateRangeWithLineCount(start, end, n)
}

func validateRangeBounds(start, end int) error {
	slog.Debug("trace", "operation", "repo.validateRangeBounds")
	if start < 1 || end < 1 {
		return fmt.Errorf("%w: line numbers must be >= 1", domain.ErrInvalidInput)
	}
	if start > end {
		return fmt.Errorf("%w: start line must be <= end line", domain.ErrInvalidInput)
	}
	return nil
}

func validateRangeWithLineCount(start, end, n int) error {
	slog.Debug("trace", "operation", "repo.validateRangeWithLineCount")
	if err := validateRangeBounds(start, end); err != nil {
		return err
	}
	if end > n {
		return fmt.Errorf("%w: line range 1-%d is past end of file (%d lines)", domain.ErrInvalidInput, end, n)
	}
	return nil
}
