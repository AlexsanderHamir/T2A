// Package postgres opens a PostgreSQL pool via GORM and migrates task schema models from package domain.
//
// Open rejects an empty or whitespace-only DSN and configures the underlying [database/sql.DB] pool
// (limits and connection lifetime) after a successful dial.
package postgres
