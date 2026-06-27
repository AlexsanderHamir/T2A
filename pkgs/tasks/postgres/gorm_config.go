package postgres

import "gorm.io/gorm"

// GORMConfigDefaults ensures cfg enables GORM translated driver errors
// (ErrDuplicatedKey, ErrForeignKeyViolated, ErrCheckConstraintViolated).
// Callers may pass nil to receive a fresh config with only defaults set.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func GORMConfigDefaults(cfg *gorm.Config) *gorm.Config {
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	cfg.TranslateError = true
	return cfg
}
