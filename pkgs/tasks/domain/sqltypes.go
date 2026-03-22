package domain

import (
	"database/sql/driver"
	"fmt"
)

func scanStringEnum[T ~string](dst *T, value any) error {
	if value == nil {
		var zero T
		*dst = zero
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*dst = T(string(v))
	case string:
		*dst = T(v)
	default:
		return fmt.Errorf("tasks: scan %T into enum", value)
	}
	return nil
}

func valueStringEnum[T ~string](s T) (driver.Value, error) {
	return string(s), nil
}

func (s *Status) Scan(value any) error     { return scanStringEnum(s, value) }
func (s Status) Value() (driver.Value, error) { return valueStringEnum(s) }

func (p *Priority) Scan(value any) error     { return scanStringEnum(p, value) }
func (p Priority) Value() (driver.Value, error) { return valueStringEnum(p) }

func (e *EventType) Scan(value any) error     { return scanStringEnum(e, value) }
func (e EventType) Value() (driver.Value, error) { return valueStringEnum(e) }

func (a *Actor) Scan(value any) error     { return scanStringEnum(a, value) }
func (a Actor) Value() (driver.Value, error) { return valueStringEnum(a) }
