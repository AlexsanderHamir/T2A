package store

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestIsDuplicateTaskPrimaryKey(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"gorm sentinel", gorm.ErrDuplicatedKey, true},
		{"sqlite tasks.id", errors.New("UNIQUE constraint failed: tasks.id"), true},
		{"sqlite wrong table", errors.New("UNIQUE constraint failed: other.id"), false},
		{"postgres pkey", errors.New(`ERROR: duplicate key value violates unique constraint "tasks_pkey" (SQLSTATE 23505)`), true},
		{"postgres other table pkey", errors.New(`ERROR: duplicate key value violates unique constraint "foo_pkey" (SQLSTATE 23505)`), false},
		{"random", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDuplicateTaskPrimaryKey(tt.err); got != tt.want {
				t.Fatalf("got %v want %v for %v", got, tt.want, tt.err)
			}
		})
	}
}
