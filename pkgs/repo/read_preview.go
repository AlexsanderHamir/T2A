package repo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

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
	fi, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%w: path is a directory", domain.ErrInvalidInput)
	}
	size := fi.Size()
	out := &FilePreview{SizeBytes: size}
	if size == 0 {
		out.LineCount = 0
		return out, nil
	}
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileReadBytes+1))
	if err != nil {
		return nil, err
	}
	truncated := false
	if int64(len(data)) > maxFileReadBytes {
		data = data[:maxFileReadBytes]
		truncated = true
	}
	out.Truncated = truncated
	if isBinaryData(data) {
		out.Binary = true
		return out, nil
	}
	if !utf8.Valid(data) {
		out.Binary = true
		return out, nil
	}
	s := string(data)
	out.Content = s
	out.LineCount = lineCountFromBytes(data)
	return out, nil
}

func isBinaryData(data []byte) bool {
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
	n := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}
