package model

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gorm.io/gorm/schema"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func gormColumnName(sf reflect.StructField) (string, bool) {
	tag := sf.Tag.Get("gorm")
	if tag == "" {
		return "", false
	}
	if tag == "-" {
		return "", false
	}
	parts := strings.Split(tag, ";")
	for _, p := range parts {
		if strings.HasPrefix(p, "column:") {
			return strings.TrimPrefix(p, "column:"), true
		}
	}
	if strings.Contains(tag, "foreignKey:") || strings.Contains(tag, "references:") {
		return "", false
	}
	return schema.NamingStrategy{}.ColumnName("", sf.Name), true
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func isAssociationField(sf reflect.StructField) bool {
	tag := sf.Tag.Get("gorm")
	if strings.Contains(tag, "foreignKey:") || strings.Contains(tag, "references:") {
		return true
	}
	if sf.Type.Kind() == reflect.Pointer && sf.Type.Elem().Kind() == reflect.Struct {
		// Pointer to another domain/model struct without a column tag is an association.
		if !strings.Contains(tag, "column:") && tag != "-" {
			return true
		}
	}
	return false
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func domainPersistedFields(domainType reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for i := 0; i < domainType.NumField(); i++ {
		sf := domainType.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := sf.Tag.Get("gorm")
		if tag == "-" {
			continue
		}
		if isAssociationField(sf) {
			continue
		}
		if _, ok := gormColumnName(sf); !ok {
			continue
		}
		out = append(out, sf)
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func modelFieldByColumn(modelType reflect.Type, column string) (reflect.StructField, bool) {
	for i := 0; i < modelType.NumField(); i++ {
		sf := modelType.Field(i)
		if col, ok := gormColumnName(sf); ok && col == column {
			return sf, true
		}
	}
	return reflect.StructField{}, false
}

// assertFieldParity reports whether every persisted domain column has a model
// counterpart with the same Go type and column name.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func assertFieldParity(pair ParityPair) error {
	dt := reflect.TypeOf(pair.Domain)
	if dt.Kind() == reflect.Pointer {
		dt = dt.Elem()
	}
	mt := reflect.TypeOf(pair.Model)
	if mt.Kind() == reflect.Pointer {
		mt = mt.Elem()
	}

	for _, df := range domainPersistedFields(dt) {
		col, _ := gormColumnName(df)
		mf, ok := modelFieldByColumn(mt, col)
		if !ok {
			return fmt.Errorf("%s: domain field %s (column %q) missing on model", pair.Name, df.Name, col)
		}
		if df.Type != mf.Type {
			return fmt.Errorf("%s: column %q type mismatch domain=%s model=%s", pair.Name, col, df.Type, mf.Type)
		}
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func sortedStrings(ss []string) []string {
	cp := append([]string(nil), ss...)
	sort.Strings(cp)
	return cp
}
