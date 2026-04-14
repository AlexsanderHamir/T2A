package repo

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// FilePreview is UTF-8 text for the @ file line-range UI, or Binary when the file should not be shown.
type FilePreview struct {
	Content   string
	Binary    bool
	Truncated bool
	SizeBytes int64
	LineCount int
}

const binarySniffBytes = 512

// ReadFilePreview reads a repo file for in-browser preview (drag-to-select line range).
// Content is capped at maxFileReadBytes; larger files return Truncated with a prefix of bytes.
// Binary or invalid UTF-8 yields Binary=true and empty Content.
func ReadFilePreview(absPath string) (*FilePreview, error) {
	slog.Debug("trace", "operation", "repo.ReadFilePreview")
	fi, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%w: path is a directory", domain.ErrInvalidInput)
	}
	size := fi.Size()
	out := &FilePreview{SizeBytes: size, Truncated: size > maxFileReadBytes}
	if size == 0 {
		out.LineCount = 0
		return out, nil
	}
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sniff, err := readBinarySniffPrefix(f)
	if err != nil {
		return nil, err
	}
	if isBinaryData(sniff) {
		out.Binary = true
		return out, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data, cappedTrunc, err := readCappedUTF8Content(f)
	if err != nil {
		return nil, err
	}
	if cappedTrunc {
		out.Truncated = true
	}
	applyBytesToPreview(out, data)
	return out, nil
}

func isBinaryData(data []byte) bool {
	slog.Debug("trace", "operation", "repo.isBinaryData")
	if len(data) == 0 {
		return false
	}
	n := binarySniffBytes
	if len(data) < n {
		n = len(data)
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}

func lineCountFromBytes(data []byte) int {
	slog.Debug("trace", "operation", "repo.lineCountFromBytes")
	n := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}
