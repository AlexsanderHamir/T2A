package kernel

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// IsDuplicateKey reports whether err is a unique-index or primary-key violation
// across GORM, SQLite, and Postgres drivers.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// Defense-in-depth when TranslateError is off or the driver returns raw text.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// IsDuplicatePrimaryKey reports whether err is a PK violation on tableName.
// tableName is matched in SQLite ("tasks.id") and Postgres ("tasks_pkey") messages.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func IsDuplicatePrimaryKey(err error, tableName string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	tableName = strings.ToLower(strings.TrimSpace(tableName))
	if tableName == "" {
		return IsDuplicateKey(err)
	}
	// Defense-in-depth for driver-specific PK constraint text.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint failed") {
		return strings.Contains(msg, tableName) && strings.Contains(msg, ".id")
	}
	if strings.Contains(msg, "duplicate key value violates unique constraint") {
		return strings.Contains(msg, tableName+"_pkey")
	}
	return false
}

// IsForeignKeyViolation reports whether err is a foreign-key constraint failure.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func IsForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return true
	}
	// Defense-in-depth when TranslateError is off or the driver returns raw text.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "foreign key")
}

// IsCheckConstraintViolation reports whether err is a check-constraint failure.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func IsCheckConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrCheckConstraintViolated) {
		return true
	}
	// Defense-in-depth when TranslateError is off or the driver returns raw text.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "constraint failed") ||
		strings.Contains(msg, "violates check constraint")
}
