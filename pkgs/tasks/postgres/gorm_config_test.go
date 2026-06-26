package postgres

import (
	"testing"

	"gorm.io/gorm"
)

func TestGORMConfigDefaults_enablesTranslateError(t *testing.T) {
	cfg := GORMConfigDefaults(nil)
	if cfg == nil || !cfg.TranslateError {
		t.Fatalf("TranslateError = %v, want true", cfg != nil && cfg.TranslateError)
	}
	preserved := GORMConfigDefaults(&gorm.Config{TranslateError: false})
	if !preserved.TranslateError {
		t.Fatal("expected GORMConfigDefaults to force TranslateError true")
	}
}
