package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
)

const (
	defaultLogEntryLimit = 100
	maxLogEntryLimit     = 500
	maxLogLineBytes      = 2 * 1024 * 1024
)

type logFileSummary struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	Modified  string `json:"modified_at"`
}

type listLogsResponse struct {
	Logs []logFileSummary `json:"logs"`
}

type logEntry struct {
	Line       int64          `json:"line"`
	Record     map[string]any `json:"record,omitempty"`
	Raw        string         `json:"raw,omitempty"`
	ParseError string         `json:"parse_error,omitempty"`
}

type logEntriesResponse struct {
	Name       string     `json:"name"`
	Offset     int64      `json:"offset"`
	Limit      int        `json:"limit"`
	NextOffset int64      `json:"next_offset"`
	HasMore    bool       `json:"has_more"`
	Entries    []logEntry `json:"entries"`
}

type logEntryFilters struct {
	level     string
	operation string
	requestID string
	query     string
	from      time.Time
	to        time.Time
	hasFrom   bool
	hasTo     bool
}

func (h *Handler) listLogs(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.list")
	const op = "logs.list"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	if strings.TrimSpace(h.logDir) == "" {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "log directory not configured")
		return
	}
	entries, err := os.ReadDir(h.logDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeJSON(w, r, op, http.StatusOK, listLogsResponse{Logs: []logFileSummary{}})
			return
		}
		slog.Error("read log directory failed", "cmd", calltrace.LogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "internal server error")
		return
	}

	logs := make([]logFileSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isTaskAPILogName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			slog.Warn("read log file info failed", "cmd", calltrace.LogCmd, "operation", op, "name", entry.Name(), "err", err)
			continue
		}
		logs = append(logs, logFileSummary{
			Name:      entry.Name(),
			SizeBytes: info.Size(),
			Modified:  info.ModTime().UTC().Format(time.RFC3339Nano),
		})
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Modified > logs[j].Modified
	})
	writeJSON(w, r, op, http.StatusOK, listLogsResponse{Logs: logs})
}

func (h *Handler) getLogEntries(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.entries")
	const op = "logs.entries"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	if strings.TrimSpace(h.logDir) == "" {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "log directory not configured")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if !isTaskAPILogName(name) {
		writeJSONError(w, r, op, http.StatusBadRequest, "invalid log file name")
		return
	}
	offset, limit, filters, err := parseLogEntriesQuery(r)
	if err != nil {
		writeJSONError(w, r, op, http.StatusBadRequest, err.Error())
		return
	}

	path := filepath.Join(h.logDir, name)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeJSONError(w, r, op, http.StatusNotFound, "not found")
			return
		}
		slog.Error("open log file failed", "cmd", calltrace.LogCmd, "operation", op, "name", name, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "internal server error")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("close log file failed", "cmd", calltrace.LogCmd, "operation", op, "name", name, "err", err)
		}
	}()

	resp, err := readLogEntries(name, file, offset, limit, filters)
	if err != nil {
		slog.Warn("read log file failed", "cmd", calltrace.LogCmd, "operation", op, "name", name, "err", err)
		writeJSONError(w, r, op, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, r, op, http.StatusOK, resp)
}

func isTaskAPILogName(name string) bool {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.isTaskAPILogName")
	if name == "" || filepath.Base(name) != name {
		return false
	}
	ok, err := filepath.Match("taskapi-*.jsonl", name)
	return err == nil && ok
}

func parseLogEntriesQuery(r *http.Request) (int64, int, logEntryFilters, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.parseQuery")
	q := r.URL.Query()
	offset, err := parseNonNegativeInt64Query(q.Get("offset"), "offset", 0)
	if err != nil {
		return 0, 0, logEntryFilters{}, err
	}
	limitRaw := strings.TrimSpace(q.Get("limit"))
	limit := int64(defaultLogEntryLimit)
	if limitRaw != "" {
		limit, err = parseNonNegativeInt64Query(limitRaw, "limit", defaultLogEntryLimit)
		if err != nil {
			return 0, 0, logEntryFilters{}, err
		}
	}
	if limit == 0 || limit > maxLogEntryLimit {
		return 0, 0, logEntryFilters{}, fmt.Errorf("limit must be 1..%d", maxLogEntryLimit)
	}
	filters := logEntryFilters{
		level:     strings.ToUpper(strings.TrimSpace(q.Get("level"))),
		operation: strings.TrimSpace(q.Get("operation")),
		requestID: strings.TrimSpace(q.Get("request_id")),
		query:     strings.ToLower(strings.TrimSpace(q.Get("q"))),
	}
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		filters.from, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return 0, 0, logEntryFilters{}, fmt.Errorf("from must be RFC3339")
		}
		filters.hasFrom = true
	}
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		filters.to, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return 0, 0, logEntryFilters{}, fmt.Errorf("to must be RFC3339")
		}
		filters.hasTo = true
	}
	return offset, int(limit), filters, nil
}

func parseNonNegativeInt64Query(raw, name string, fallback int64) (int64, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.parseNonNegativeInt64Query")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	if len(raw) > 18 {
		return 0, fmt.Errorf("%s value too long", name)
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return v, nil
}

func readLogEntries(name string, file *os.File, offset int64, limit int, filters logEntryFilters) (logEntriesResponse, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.readEntries")
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLogLineBytes)
	resp := logEntriesResponse{
		Name:    name,
		Offset:  offset,
		Limit:   limit,
		Entries: []logEntry{},
	}
	var lineNo int64
	for scanner.Scan() {
		lineNo++
		if lineNo <= offset {
			continue
		}
		entry := parseLogLine(lineNo, scanner.Text())
		if !entryMatchesFilters(entry, filters) {
			resp.NextOffset = lineNo
			continue
		}
		if len(resp.Entries) >= limit {
			resp.HasMore = true
			resp.NextOffset = lineNo - 1
			return resp, nil
		}
		resp.Entries = append(resp.Entries, entry)
		resp.NextOffset = lineNo
	}
	if err := scanner.Err(); err != nil {
		return resp, fmt.Errorf("read log lines: %w", err)
	}
	return resp, nil
}

func parseLogLine(lineNo int64, raw string) logEntry {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.parseLine")
	var record map[string]any
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return logEntry{Line: lineNo, Raw: raw, ParseError: err.Error()}
	}
	return logEntry{Line: lineNo, Record: record}
}

func entryMatchesFilters(entry logEntry, filters logEntryFilters) bool {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.entryMatchesFilters")
	if filters.query != "" {
		var haystack string
		if entry.Record != nil {
			b, _ := json.Marshal(entry.Record)
			haystack = strings.ToLower(string(b))
		} else {
			haystack = strings.ToLower(entry.Raw)
		}
		if !strings.Contains(haystack, filters.query) {
			return false
		}
	}
	if entry.Record == nil {
		return filters.level == "" && filters.operation == "" && filters.requestID == "" && !filters.hasFrom && !filters.hasTo
	}
	if filters.level != "" && strings.ToUpper(stringField(entry.Record, "level")) != filters.level {
		return false
	}
	if filters.operation != "" && stringField(entry.Record, "operation") != filters.operation {
		return false
	}
	if filters.requestID != "" && stringField(entry.Record, "request_id") != filters.requestID {
		return false
	}
	if filters.hasFrom || filters.hasTo {
		t, err := time.Parse(time.RFC3339Nano, stringField(entry.Record, "time"))
		if err != nil {
			return false
		}
		if filters.hasFrom && t.Before(filters.from) {
			return false
		}
		if filters.hasTo && t.After(filters.to) {
			return false
		}
	}
	return true
}

func stringField(record map[string]any, key string) string {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.logs.stringField")
	v, ok := record[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
