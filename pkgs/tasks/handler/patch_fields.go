package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
)

// patchParentField decodes optional JSON parent_id: omitted (no change), null (clear), or string.
type patchParentField struct {
	Defined bool
	Clear   bool
	SetID   string
}

// patchProjectField decodes optional JSON project_id: omitted (no change),
// null (clear), or non-empty string.
type patchProjectField struct {
	Defined bool
	Clear   bool
	SetID   string
}

func (p *patchProjectField) UnmarshalJSON(b []byte) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.patchProjectField.UnmarshalJSON")
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}
	p.Defined = true
	if bytes.Equal(b, []byte("null")) {
		p.Clear = true
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("project_id must not be empty")
	}
	p.SetID = s
	return nil
}

func (p *patchParentField) UnmarshalJSON(b []byte) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.patchParentField.UnmarshalJSON")
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}
	p.Defined = true
	if bytes.Equal(b, []byte("null")) {
		p.Clear = true
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("parent_id must not be empty")
	}
	p.SetID = s
	return nil
}

// patchPickupNotBeforeField decodes optional JSON pickup_not_before
// (RFC3339 UTC). Three on-the-wire shapes, three semantics:
//
//   - field omitted from JSON: no change (Defined == false).
//   - JSON null OR explicit empty string ""  : clear the column
//     (Defined == true, Clear == true). Empty string is the
//     SchedulePicker's "no schedule" emit value, kept symmetric with
//     the documented JSON null contract so SPA code that renders an
//     `<input type="datetime-local">` cleared by the user (which fires
//     the empty string) doesn't have to special-case JSON null.
//   - JSON string parseable as RFC3339: Defined == true, Set is the
//     parsed UTC time. Pre-2000 sentinel is rejected with a stable
//     message so the documented "0001-01-01T00:00:00Z" zero value
//     can't sneak in as "no schedule" via the type-default path.
//
// Errors here surface to the caller as `domain.ErrInvalidInput`-class
// 400s via the standard handler envelope.
type patchPickupNotBeforeField struct {
	Defined bool
	Clear   bool
	Set     time.Time
}

// pickupNotBeforeMinAllowed is the documented sentinel boundary.
// Anything before this is an obvious zero-value or accidentally
// shifted timestamp; the column is RFC3339-only on the wire and the
// product never wants pre-2000 schedules.
var pickupNotBeforeMinAllowed = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func (p *patchPickupNotBeforeField) UnmarshalJSON(b []byte) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.patchPickupNotBeforeField.UnmarshalJSON")
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}
	p.Defined = true
	if bytes.Equal(b, []byte("null")) {
		p.Clear = true
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return errors.New("pickup_not_before must be null or an RFC3339 string")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		// PATCH-only "explicit clear via empty string" path. The
		// taskCreateJSON.PickupNotBefore call site is responsible
		// for rejecting empty on create — see resolvePickupNotBeforeForCreate.
		p.Clear = true
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("pickup_not_before must be RFC3339 (e.g. 2026-04-19T15:30:00Z): %v", err)
	}
	if t.Before(pickupNotBeforeMinAllowed) {
		return errors.New("pickup_not_before must be on or after 2000-01-01T00:00:00Z")
	}
	p.Set = t.UTC()
	return nil
}
