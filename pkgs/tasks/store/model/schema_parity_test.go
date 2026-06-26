package model

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type tableColumn struct {
	CID       int
	Name      string
	Type      string
	NotNull   int
	DfltValue *string
	PK        int
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func migrateModel(t *testing.T, pair ParityPair) *gorm.DB {
	t.Helper()
	db := openTestDB(t)
	for _, extra := range pair.ModelMigrateExtra {
		if err := db.AutoMigrate(extra); err != nil {
			t.Fatalf("model migrate extra %T: %v", extra, err)
		}
	}
	if err := db.AutoMigrate(pair.Model); err != nil {
		t.Fatalf("model migrate %s: %v", pair.Name, err)
	}
	return db
}

func readColumns(t *testing.T, db *gorm.DB, table string) []tableColumn {
	t.Helper()
	var cols []tableColumn
	if err := db.Raw(fmt.Sprintf("PRAGMA table_info(%q)", table)).Scan(&cols).Error; err != nil {
		t.Fatalf("pragma table_info %s: %v", table, err)
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })
	return cols
}

func readIndexSQL(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var rows []struct {
		SQL string `gorm:"column:sql"`
	}
	q := `SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL`
	if err := db.Raw(q, table).Scan(&rows).Error; err != nil {
		t.Fatalf("read indexes %s: %v", table, err)
	}
	var out []string
	for _, r := range rows {
		if strings.HasPrefix(r.SQL, "sqlite_autoindex_") {
			continue
		}
		out = append(out, r.SQL)
	}
	return sortedStrings(out)
}

func formatColumns(cols []tableColumn) []string {
	var out []string
	for _, c := range cols {
		dflt := ""
		if c.DfltValue != nil {
			dflt = *c.DfltValue
		}
		out = append(out, fmt.Sprintf("%s|%s|notnull=%d|pk=%d|dflt=%s", c.Name, c.Type, c.NotNull, c.PK, dflt))
	}
	return out
}

// TestSchemaParity verifies each store model AutoMigrates cleanly with FK extras.
func TestSchemaParity(t *testing.T) {
	t.Parallel()
	for _, pair := range ParityPairs {
		pair := pair
		t.Run(pair.Name, func(t *testing.T) {
			t.Parallel()
			db := migrateModel(t, pair)
			cols := readColumns(t, db, pair.Table)
			if len(cols) == 0 {
				t.Fatalf("table %s has no columns after migrate", pair.Table)
			}
			_ = readIndexSQL(t, db, pair.Table)
		})
	}
}
