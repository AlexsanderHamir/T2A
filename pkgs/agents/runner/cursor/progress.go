package cursor

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"path"
	"sort"
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
	Type     string          `json:"type,omitempty"`
	Subtype  string          `json:"subtype,omitempty"`
	Model    string          `json:"model,omitempty"`
	Name     string          `json:"name,omitempty"`
	Tool     string          `json:"tool,omitempty"`
	Message  progressMessage `json:"message,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	ToolCall json.RawMessage `json:"tool_call,omitempty"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
		nestedTool, nestedInput := toolCallDetails(line.ToolCall)
		tool := firstNonEmpty(line.Name, line.Tool, nestedTool)
		subtype := strings.TrimSpace(line.Subtype)
		input := firstRawMessage(line.Input, nestedInput)
		msg := toolProgressMessage(tool, subtype, input)
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func firstRawMessage(values ...json.RawMessage) json.RawMessage {
	for _, v := range values {
		if len(bytes.TrimSpace(v)) > 0 {
			return v
		}
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func toolCallDetails(raw json.RawMessage) (string, json.RawMessage) {
	if len(raw) == 0 {
		return "", nil
	}
	var calls map[string]json.RawMessage
	if err := json.Unmarshal(raw, &calls); err != nil || len(calls) == 0 {
		return "", nil
	}
	for _, key := range preferredToolCallKeys() {
		if body, ok := calls[key]; ok {
			return toolCallBodyDetails(key, body)
		}
	}
	keys := make([]string, 0, len(calls))
	for key := range calls {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return toolCallBodyDetails(keys[0], calls[keys[0]])
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func preferredToolCallKeys() []string {
	return []string{
		"readToolCall",
		"writeToolCall",
		"editToolCall",
		"grepToolCall",
		"ripgrepToolCall",
		"globToolCall",
		"shellToolCall",
		"deleteToolCall",
		"function",
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func toolCallBodyDetails(key string, raw json.RawMessage) (string, json.RawMessage) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return toolNameFromCallKey(key), nil
	}
	if key == "function" {
		if name := rawString(body["name"]); name != "" {
			return name, functionArguments(body["arguments"])
		}
	}
	return toolNameFromCallKey(key), body["args"]
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func toolNameFromCallKey(key string) string {
	switch strings.TrimSpace(key) {
	case "readToolCall":
		return "ReadFile"
	case "writeToolCall":
		return "WriteFile"
	case "editToolCall":
		return "EditFile"
	case "grepToolCall", "ripgrepToolCall":
		return "rg"
	case "globToolCall":
		return "Glob"
	case "shellToolCall":
		return "Shell"
	case "deleteToolCall":
		return "Delete"
	default:
		return strings.TrimSuffix(strings.TrimSpace(key), "ToolCall")
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func functionArguments(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if json.Valid(raw) && bytes.TrimSpace(raw)[0] == '{' {
		return raw
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	return raw
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
		return "Starting " + label
	case cursorSubtypeCompleted, cursorSubtypeSuccess, cursorSubtypeDone:
		return "Finishing " + label
	case cursorSubtypeFailed, cursorSubtypeError:
		return "Failing " + label
	default:
		return label
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
	case strings.Contains(name, "applypatch") || strings.Contains(name, "edit"):
		return pathActionSummary("Editing", fields)
	case strings.Contains(name, "write"):
		return pathActionSummary("Writing", fields)
	case strings.Contains(name, "delete"):
		return pathActionSummary("Deleting", fields)
	case strings.Contains(name, "shell") || strings.Contains(name, "terminal") || strings.Contains(name, "bash"):
		return shellSummary(fields)
	default:
		return genericInputSummary(fields)
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func readFileSummary(fields map[string]any) string {
	p := pathLabel(inputField(fields, "path", "file", "target_file", "targetFile"))
	if p == "" {
		return ""
	}
	if r := lineRange(fields); r != "" {
		return clipProgressSummary("Reading " + p + " " + r)
	}
	return clipProgressSummary("Reading " + p)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func searchFilesSummary(fields map[string]any) string {
	pattern := inputField(fields, "glob_pattern", "globPattern", "pattern", "query", "q")
	scope := scopeLabel(inputField(fields, "target_directory", "targetDirectory", "path", "directory", "dir"))
	if pattern != "" && scope != "" {
		return clipProgressSummary("Searching for " + pattern + " in " + scope)
	}
	if pattern != "" {
		return clipProgressSummary("Searching for " + pattern)
	}
	if scope != "" {
		return clipProgressSummary("Searching files in " + scope)
	}
	return clipProgressSummary("Searching files")
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ripgrepSummary(fields map[string]any) string {
	pattern := inputField(fields, "pattern", "query", "q", "glob")
	scope := scopeLabel(inputField(fields, "path", "target_directory", "targetDirectory", "directory", "dir"))
	if pattern == "" {
		pattern = "text"
	}
	if scope != "" {
		return clipProgressSummary("Searching " + pattern + " in " + scope)
	}
	return clipProgressSummary("Searching " + pattern)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func editSummary(fields map[string]any) string {
	p := pathLabel(inputField(fields, "target_file", "targetFile", "path", "file"))
	if p == "" {
		return ""
	}
	return clipProgressSummary("Editing " + p)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func pathActionSummary(action string, fields map[string]any) string {
	p := pathLabel(inputField(fields, "path", "target_file", "targetFile", "file"))
	if p == "" {
		return ""
	}
	return clipProgressSummary(action + " " + p)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func genericInputSummary(fields map[string]any) string {
	if inputField(fields, "glob_pattern", "globPattern") != "" {
		return searchFilesSummary(fields)
	}
	if p := pathLabel(inputField(fields, "path", "target_file", "targetFile", "file")); p != "" {
		if r := lineRange(fields); r != "" {
			return clipProgressSummary("Reading " + p + " " + r)
		}
		return clipProgressSummary(p)
	}
	if query := inputField(fields, "query", "pattern", "glob_pattern", "globPattern"); query != "" {
		return clipProgressSummary("Searching " + query)
	}
	return ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// inputField returns the first non-empty string value for any of the given
// keys. Cursor stream-json uses camelCase in nested tool_call args while
// older flat input blobs use snake_case — accept both so summaries stay stable.
func inputField(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := stringField(fields, key); s != "" {
			return s
		}
	}
	return ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func intString(n int64) string {
	return strconv.FormatInt(n, 10)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func clipProgressSummary(s string) string {
	return clipSummaryRunes(strings.Join(strings.Fields(s), " "), 80)
}
