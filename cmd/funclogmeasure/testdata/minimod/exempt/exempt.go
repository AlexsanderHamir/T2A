package exempt

import (
	"database/sql/driver"
	"fmt"
)

type errVal struct{}

func (e errVal) Error() string { return "boom" }

type stringerVal struct{}

func (s stringerVal) String() string { return "ok" }

type enum string

func (e *enum) Scan(src []byte) error {
	if src == nil {
		return nil
	}
	*e = enum(src)
	return nil
}

func (e enum) Value() (driver.Value, error) {
	return string(e), nil
}

type model struct{}

func (model) TableName() string {
	return "widgets"
}

var _ fmt.Stringer = stringerVal{}
