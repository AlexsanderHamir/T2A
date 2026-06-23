package taskapiconfig

import "testing"

func TestMigrateEnabled_flagWins(t *testing.T) {
	t.Setenv(EnvMigrate, "")
	if !MigrateEnabled(true) {
		t.Fatal("flag true should enable migrate")
	}
}

func TestMigrateEnabled_env(t *testing.T) {
	t.Setenv(EnvMigrate, "1")
	if !MigrateEnabled(false) {
		t.Fatal("HAMIX_MIGRATE=1 should enable migrate")
	}
	t.Setenv(EnvMigrate, "")
	if MigrateEnabled(false) {
		t.Fatal("unset env should not enable migrate")
	}
}
