// Package envload loads a dotenv file and enforces that DATABASE_URL is set for commands
// that need Postgres (for example the taskapi binary in this module).
//
// [Load] uses the process working directory, walks parent directories until it finds go.mod,
// and treats that directory as the repository root. If envFileOverride is non-empty, that
// path is used instead (after filepath.Clean). The chosen file is loaded with
// godotenv.Overload, which replaces existing environment variables.
//
// After load, if DATABASE_URL is empty, Load returns an error naming the path that was read.
// Callers typically log and exit non-zero.
//
// This package is internal to the module; importers use the full module path
// github.com/AlexsanderHamir/T2A/internal/envload.
package envload
