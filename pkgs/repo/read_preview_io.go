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
		// The byte cap may have landed inside a multi-byte UTF-8 rune
		// (UTF-8 encodes one rune in up to UTFMax=4 bytes). Walk back
		// from the cut to the last full rune-start byte and drop any
		// trailing partial encoding; otherwise applyBytesToPreview's
		// utf8.Valid check would mis-label an otherwise-valid text
		// file as binary purely because the cap split a rune.
		data = trimTrailingPartialRune(data)
	}
	return data, truncated, nil
}

// trimTrailingPartialRune removes a dangling partial UTF-8 sequence at the
// tail of data, if any. It scans back at most UTFMax-1 bytes (the longest
// possible incomplete encoding) for a rune-start byte, decodes it, and
// strips the leader plus any continuation bytes when the encoding is
// shorter than the bytes available. Inputs that already end on a rune
// boundary are returned unchanged. Inputs that are not valid UTF-8 in
// their interior are also returned unchanged — this helper only fixes the
// truncation seam, not pre-existing invalidity.
func trimTrailingPartialRune(data []byte) []byte {
	slog.Debug("trace", "operation", "repo.trimTrailingPartialRune", "bytes", len(data))
	if len(data) == 0 {
		return data
	}
	// Look back at most UTFMax-1 bytes for a rune-start (i.e. not a
	// continuation byte 0x80-0xBF).
	limit := len(data) - utf8.UTFMax
	if limit < 0 {
		limit = 0
	}
	for i := len(data) - 1; i >= limit; i-- {
		if !utf8.RuneStart(data[i]) {
			continue
		}
		// Determine the expected rune length from the leader byte.
		var runeLen int
		switch b := data[i]; {
		case b < 0x80:
			runeLen = 1
		case b < 0xC2:
			// 0x80-0xBF are continuation bytes (filtered above);
			// 0xC0-0xC1 are illegal UTF-8 leaders. Treat as opaque
			// and bail without trimming.
			return data
		case b < 0xE0:
			runeLen = 2
		case b < 0xF0:
			runeLen = 3
		case b < 0xF5:
			runeLen = 4
		default:
			// 0xF5-0xFF are illegal UTF-8 leaders; bail.
			return data
		}
		if i+runeLen > len(data) {
			return data[:i]
		}
		return data
	}
	return data
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
