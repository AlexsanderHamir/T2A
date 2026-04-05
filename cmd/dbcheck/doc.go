// Command dbcheck verifies that a .env file can be loaded, DATABASE_URL is set, Postgres is
// reachable (ping), and optionally migrates task tables.
//
// It resolves the dotenv path the same way as internal/envload (walk from cwd to go.mod,
// then <repo-root>/.env, or use -env when set), loads with godotenv.Overload, and requires a
// non-empty DATABASE_URL. It does not import the envload package; behavior is intended to match.
//
// Flags (see also -h):
//
//	-env string     path to .env (default: <repo-root>/.env)
//	-migrate        run GORM AutoMigrate after connecting
//
// The first Info line includes a version field (same build metadata as taskapi health JSON).
//
// On success it logs and exits 0; on failure it logs and exits 1. Ping uses a 30s deadline;
// with -migrate, postgres.Migrate (pkgs/tasks/postgres) runs under a separate 120s deadline,
// same tables and migrate wall clock as taskapi at startup.
package main
