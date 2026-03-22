// Package domain defines task types ([Task], [TaskEvent]), enums ([Status], [Priority],
// [EventType], [Actor]), sentinel errors ([ErrNotFound], [ErrInvalidInput]), and database
// driver Scan/Value methods for those enums.
//
// Persistence is implemented in package store (github.com/AlexsanderHamir/T2A/pkgs/tasks/store).
// HTTP routes in package handler (github.com/AlexsanderHamir/T2A/pkgs/tasks/handler).
// Open database pool and AutoMigrate in package postgres (github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres).
//
// Tests for enum scanning live in sqltypes_test.go. See the parent module path
// github.com/AlexsanderHamir/T2A/pkgs/tasks for a full subsystem overview.
package domain
