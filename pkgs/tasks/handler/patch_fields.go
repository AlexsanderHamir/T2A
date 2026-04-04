package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// patchParentField decodes optional JSON parent_id: omitted (no change), null (clear), or string.
type patchParentField struct {
	Defined bool
	Clear   bool
	SetID   string
}

func (p *patchParentField) UnmarshalJSON(b []byte) error {
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
