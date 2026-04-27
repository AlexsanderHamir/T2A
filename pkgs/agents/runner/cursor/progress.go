package cursor

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"path"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
)

type progressMessage struct {
	Role    string            `json:"role,omitempty"`
	Content []progressContent `json:"content,omitempty"`
}

type progressContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type progressEventLine struct {
	Type    string          `json:"type,omitempty"`
	Subtype string          `json:"subtype,omitempty"`
	Model   string          `json:"model,omitempty"`
	Name    string          `json:"name,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Message progressMessage `json:"message,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
}

func emitProgressFromLine(onProgress func(runner.ProgressEvent), raw []byte, homePaths []string) {
	if onProgress == nil {
		return
	}
	ev, ok := progressFromLine(raw, homePaths)
	if !ok {
		return
	}
	defer func() {
		_ = recover()
	}()
	onProgress(ev)
}

func progressFromLine(raw []byte, homePaths []string) (runner.ProgressEvent, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return runner.ProgressEvent{}, false
	}
	var line progressEventLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return runner.ProgressEvent{}, false
	}
	switch line.Type {
	case cursorEventSystem:
		if line.Subtype == cursorSubtypeInit && strings.TrimSpace(line.Model) != "" {
			return runner.ProgressEvent{
				Kind:    cursorEventSystem,
				Subtype: cursorSubtypeInit,
				Message: "Using " + strings.TrimSpace(line.Model),
				Payload: progressPayload(raw, homePaths),
			}, true
		}
	case cursorEventAssistant:
		msg := clipSummaryRunes(redact(strings.TrimSpace(textContent(line.Message.Content)), homePaths), limits.ProgressSummaryRunes)
		if msg != "" {
			return runner.ProgressEvent{Kind: cursorEventAssistant, Message: msg, Payload: progressPayload(raw, homePaths)}, true
		}
	case cursorEventToolCall:
		tool := firstNonEmpty(line.Name, line.Tool)
		subtype := strings.TrimSpace(line.Subtype)
		msg := toolProgressMessage(tool, subtype, line.Input)
		return runner.ProgressEvent{
			Kind:    cursorEventToolCall,
			Subtype: subtype,
			Tool:    tool,
			Message: msg,
			Payload: progressPayload(raw, homePaths),
		}, true
	}
	return runner.ProgressEvent{}, false
}

func progressPayload(raw []byte, homePaths []string) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.progressPayload", "bytes", len(raw))
	redacted := []byte(redact(string(raw), homePaths))
	if !json.Valid(redacted) {
		return nil
	}
	return json.RawMessage(redacted)
}

func textContent(parts []progressContent) string {
	var b strings.Builder
	for _, part := range parts {
		if part.Type != cursorContentText {
			continue
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func toolProgressMessage(tool, subtype string, input json.RawMessage) string {
	if msg := toolInputSummary(tool, input); msg != "" {
		return msg
	}
	label := strings.TrimSpace(tool)
	if label == "" {
		label = "tool"
	}
	switch subtype {
	case cursorSubtypeStarted, cursorSubtypeStart:
		return "Started " + label
	case cursorSubtypeCompleted, cursorSubtypeSuccess, cursorSubtypeDone:
		return "Finished " + label
	case cursorSubtypeFailed, cursorSubtypeError:
		return "Failed " + label
	default:
		return label
	}
}

func toolInputSummary(tool string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var fields map[string]any
	if err := json.Unmarshal(input, &fields); err != nil {
		return ""
	}
	name := strings.ToLower(firstNonEmpty(tool, stringField(fields, "tool"), stringField(fields, "name")))
	switch {
	case strings.Contains(name, "readfile") || strings.Contains(name, "read_file"):
		return readFileSummary(fields)
	case strings.Contains(name, "glob"):
		return searchFilesSummary(fields)
	case name == "rg" || strings.Contains(name, "ripgrep") || strings.Contains(name, "grep"):
		return ripgrepSummary(fields)
	case strings.Contains(name, "applypatch") || strings.Contains(name, "edit") || strings.Contains(name, "write"):
		return editSummary(fields)
	case strings.Contains(name, "delete"):
		return pathActionSummary("Delete", fields)
	case strings.Contains(name, "shell") || strings.Contains(name, "terminal") || strings.Contains(name, "bash"):
		return shellSummary(fields)
	default:
		return genericInputSummary(fields)
	}
}

func readFileSummary(fields map[string]any) string {
	p := pathLabel(firstNonEmpty(stringField(fields, "path"), stringField(fields, "file"), stringField(fields, "target_file")))
	if p == "" {
		return ""
	}
	if r := lineRange(fields); r != "" {
		return clipProgressSummary("Read " + p + " " + r)
	}
	return clipProgressSummary("Read " + p)
}

func searchFilesSummary(fields map[string]any) string {
	pattern := firstNonEmpty(stringField(fields, "glob_pattern"), stringField(fields, "pattern"), stringField(fields, "query"), stringField(fields, "q"))
	if pattern == "" {
		pattern = "files"
	}
	scope := scopeLabel(firstNonEmpty(stringField(fields, "target_directory"), stringField(fields, "path"), stringField(fields, "directory"), stringField(fields, "dir")))
	if scope != "" {
		return clipProgressSummary("Searching files " + pattern + " in " + scope)
	}
	return clipProgressSummary("Searching files " + pattern)
}

func ripgrepSummary(fields map[string]any) string {
	pattern := firstNonEmpty(stringField(fields, "pattern"), stringField(fields, "query"), stringField(fields, "q"), stringField(fields, "glob"))
	scope := scopeLabel(firstNonEmpty(stringField(fields, "path"), stringField(fields, "target_directory"), stringField(fields, "directory"), stringField(fields, "dir")))
	if pattern == "" {
		pattern = "text"
	}
	if scope != "" {
		return clipProgressSummary("Searching " + pattern + " in " + scope)
	}
	return clipProgressSummary("Searching " + pattern)
}

func editSummary(fields map[string]any) string {
	p := pathLabel(firstNonEmpty(stringField(fields, "target_file"), stringField(fields, "path"), stringField(fields, "file")))
	if p == "" {
		return ""
	}
	return clipProgressSummary("Editing " + p)
}

func pathActionSummary(action string, fields map[string]any) string {
	p := pathLabel(firstNonEmpty(stringField(fields, "path"), stringField(fields, "target_file"), stringField(fields, "file")))
	if p == "" {
		return ""
	}
	return clipProgressSummary(action + " " + p)
}

func shellSummary(fields map[string]any) string {
	desc := strings.TrimSpace(stringField(fields, "description"))
	if desc != "" {
		return clipProgressSummary(desc)
	}
	command := firstNonEmpty(stringField(fields, "command"), stringField(fields, "cmd"))
	if command == "" {
		return ""
	}
	return clipProgressSummary("Running " + shellCommandLabel(command))
}

func genericInputSummary(fields map[string]any) string {
	if stringField(fields, "glob_pattern") != "" {
		return searchFilesSummary(fields)
	}
	if p := pathLabel(firstNonEmpty(stringField(fields, "path"), stringField(fields, "target_file"), stringField(fields, "file"))); p != "" {
		if r := lineRange(fields); r != "" {
			return clipProgressSummary("Read " + p + " " + r)
		}
		return clipProgressSummary(p)
	}
	if query := firstNonEmpty(stringField(fields, "query"), stringField(fields, "pattern"), stringField(fields, "glob_pattern")); query != "" {
		return clipProgressSummary("Searching " + query)
	}
	return ""
}

func stringField(fields map[string]any, key string) string {
	v, ok := fields[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

func pathLabel(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" {
		return ""
	}
	base := path.Base(p)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func scopeLabel(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" {
		return ""
	}
	base := path.Base(strings.TrimSuffix(p, "/"))
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func lineRange(fields map[string]any) string {
	start, ok := numericField(fields, "offset")
	if !ok {
		start, ok = numericField(fields, "start")
	}
	if !ok || start <= 0 {
		return ""
	}
	if end, ok := numericField(fields, "end"); ok && end >= start {
		return "L" + intString(start) + "-" + intString(end)
	}
	if limit, ok := numericField(fields, "limit"); ok && limit > 0 {
		return "L" + intString(start) + "-" + intString(start+limit-1)
	}
	return "L" + intString(start)
}

func numericField(fields map[string]any, key string) (int64, bool) {
	switch v := fields[key].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

func intString(n int64) string {
	return strconv.FormatInt(n, 10)
}

func shellCommandLabel(command string) string {
	command = strings.Join(strings.Fields(command), " ")
	if command == "" {
		return ""
	}
	parts := strings.Fields(command)
	if len(parts) > 4 {
		return strings.Join(parts[:4], " ") + "..."
	}
	return command
}

func clipProgressSummary(s string) string {
	return clipSummaryRunes(strings.Join(strings.Fields(s), " "), 80)
}
