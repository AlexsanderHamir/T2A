package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
)

func TestStore_Ping_ok(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStore_Ready_ok(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	if err := s.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStore_Ready_fails_when_db_closed(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Ready(context.Background()); err == nil {
		t.Fatal("expected error after close")
	}
}
