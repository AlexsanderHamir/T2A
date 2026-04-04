package domain

import (
	"context"
	"database/sql/driver"
	"fmt"
	"log/slog"
)

func scanStringEnum[T ~string](dst *T, value any) error {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.scanStringEnum")
	}
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
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.valueStringEnum")
	}
	return string(s), nil
}

func (s *Status) Scan(value any) error {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.Status.Scan")
	}
	return scanStringEnum(s, value)
}

func (s Status) Value() (driver.Value, error) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.Status.Value")
	}
	return valueStringEnum(s)
}

func (p *Priority) Scan(value any) error {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.Priority.Scan")
	}
	return scanStringEnum(p, value)
}

func (p Priority) Value() (driver.Value, error) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.Priority.Value")
	}
	return valueStringEnum(p)
}

func (e *EventType) Scan(value any) error {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.EventType.Scan")
	}
	return scanStringEnum(e, value)
}

func (e EventType) Value() (driver.Value, error) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.EventType.Value")
	}
	return valueStringEnum(e)
}

func (a *Actor) Scan(value any) error {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.Actor.Scan")
	}
	return scanStringEnum(a, value)
}

func (a Actor) Value() (driver.Value, error) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.Actor.Value")
	}
	return valueStringEnum(a)
}
