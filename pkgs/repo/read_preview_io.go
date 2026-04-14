package repo

import (
	"io"
	"log/slog"
	"os"
	"unicode/utf8"
)

func readBinarySniffPrefix(f *os.File) ([]byte, error) {
	slog.Debug("trace", "operation", "repo.readBinarySniffPrefix")
	sniff := make([]byte, binarySniffBytes)
	nSniff, err := io.ReadFull(f, sniff)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return sniff[:nSniff], nil
}

func readCappedUTF8Content(f *os.File) (data []byte, truncated bool, err error) {
	slog.Debug("trace", "operation", "repo.readCappedUTF8Content")
	data, err = io.ReadAll(io.LimitReader(f, maxFileReadBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxFileReadBytes {
		data = data[:maxFileReadBytes]
		truncated = true
	}
	return data, truncated, nil
}

func applyBytesToPreview(out *FilePreview, data []byte) {
	slog.Debug("trace", "operation", "repo.applyBytesToPreview")
	if isBinaryData(data) {
		out.Binary = true
		return
	}
	if !utf8.Valid(data) {
		out.Binary = true
		return
	}
	out.Content = string(data)
	out.LineCount = lineCountFromBytes(data)
}
