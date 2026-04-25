package cursor

import (
	"bytes"
	"encoding/json"
	"log/slog"
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
		msg := toolProgressMessage(tool, subtype)
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

func toolProgressMessage(tool, subtype string) string {
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
