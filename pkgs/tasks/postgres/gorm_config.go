package postgres

import "gorm.io/gorm"

// GORMConfigDefaults ensures cfg enables GORM translated driver errors
// (ErrDuplicatedKey, ErrForeignKeyViolated, ErrCheckConstraintViolated).
// Callers may pass nil to receive a fresh config with only defaults set.
func GORMConfigDefaults(cfg *gorm.Config) *gorm.Config {
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	cfg.TranslateError = true
	return cfg
}
