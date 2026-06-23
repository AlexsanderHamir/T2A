package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestCheckSchemaDrift_pendingWithoutRow(t *testing.T) {
	db := openSchemaTestDB(t)
	report, err := CheckSchemaDrift(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != SchemaDriftPending {
		t.Fatalf("status=%q want pending", report.Status)
	}
	if report.CodeRevision != SchemaRevision {
		t.Fatalf("code_revision=%d want %d", report.CodeRevision, SchemaRevision)
	}
	if report.DBRevision != 0 {
		t.Fatalf("db_revision=%d want 0", report.DBRevision)
	}
	if !report.FailsReadiness() {
		t.Fatal("expected readiness failure")
	}
}

func TestCheckSchemaDrift_okAfterRecord(t *testing.T) {
	db := openSchemaTestDB(t)
	ctx := context.Background()
	if err := RecordSchemaRevision(ctx, db, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	report, err := CheckSchemaDrift(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != SchemaDriftOK {
		t.Fatalf("status=%q want ok", report.Status)
	}
	if report.DBRevision != SchemaRevision {
		t.Fatalf("db_revision=%d want %d", report.DBRevision, SchemaRevision)
	}
}

func TestCheckSchemaDrift_downgradeWhenDBAhead(t *testing.T) {
	db := openSchemaTestDB(t)
	ctx := context.Background()
	if err := db.WithContext(ctx).AutoMigrate(&SchemaMeta{}); err != nil {
		t.Fatal(err)
	}
	ahead := SchemaRevision + 1
	if err := db.WithContext(ctx).Create(&SchemaMeta{
		ID:        schemaMetaRowID,
		Revision:  ahead,
		AppliedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := CheckSchemaDrift(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != SchemaDriftDowngrade {
		t.Fatalf("status=%q want downgrade", report.Status)
	}
	if report.DBRevision != ahead {
		t.Fatalf("db_revision=%d want %d", report.DBRevision, ahead)
	}
}
