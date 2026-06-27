package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

// DefaultMigrateTimeout is the recommended upper bound for [Migrate] from operators
// (taskapi startup, dbcheck -migrate). Tests and fast local runs may use a shorter deadline or
// [context.Background] when AutoMigrate is expected to finish quickly.
const DefaultMigrateTimeout = 2 * time.Minute

// DefaultPingTimeout is the recommended upper bound for the first successful [database/sql.DB.PingContext]
// from operator CLIs (dbcheck). Long-lived servers may omit an explicit ping or use their own probe policy.
const DefaultPingTimeout = 30 * time.Second

func configureSQLPool(sqldb *sql.DB) {
	slog.Debug("trace", "operation", "postgres.configureSQLPool")
	sqldb.SetMaxOpenConns(defaultMaxOpenConns)
	sqldb.SetMaxIdleConns(defaultMaxIdleConns)
	sqldb.SetConnMaxLifetime(defaultConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(defaultConnMaxIdleTime)
}

// Open returns a GORM DB connected to PostgreSQL using the given DSN.
func Open(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	slog.Debug("trace", "operation", "postgres.Open")
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("postgres open: %w", errEmptyDSN)
	}
	dsn = ensureQueryExecModeSimpleProtocol(dsn)
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	cfg = GORMConfigDefaults(cfg)
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqldb, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm sql db: %w", err)
	}
	configureSQLPool(sqldb)
	return db, nil
}

// Migrate runs AutoMigrate for domain.Task and domain.TaskEvent (works with any GORM dialector, e.g. tests on SQLite).
func Migrate(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.Migrate")
	db = db.Session(&gorm.Session{
		Logger: gormlogger.NewSlogLogger(slog.Default(), gormlogger.Config{
			LogLevel:                  gormlogger.Warn,
			SlowThreshold:             slowQueryThresholdForGORM(),
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
	})
	if db.Dialector != nil && db.Dialector.Name() == "postgres" {
		if err := db.WithContext(ctx).Exec(`ALTER TABLE project_context_items DROP CONSTRAINT IF EXISTS chk_project_context_kind`).Error; err != nil {
			return fmt.Errorf("drop project context kind constraint: %w", err)
		}
	}
	if err := migrateExpandFixedWorktreeBranch(ctx, db); err != nil {
		return fmt.Errorf("expand fixed worktree branch: %w", err)
	}
	if err := db.WithContext(ctx).AutoMigrate(
		&domain.Project{},
		&domain.Task{},
		&domain.TaskDependency{},
		&domain.TaskEvent{},
		&domain.TaskChecklistItem{},
		&domain.TaskChecklistItemCommand{},
		&domain.TaskChecklistCompletion{},
		&domain.TaskDraft{},
		&domain.TaskTemplate{},
		&domain.TaskCycle{},
		&domain.TaskCyclePhase{},
		&domain.TaskCycleStreamEvent{},
		&domain.TaskCycleCriteriaReport{},
		&domain.TaskCycleVerifyReport{},
		&domain.TaskCycleCommandRun{},
		&domain.TaskCycleCommit{},
		&domain.ProjectContextItem{},
		&domain.ProjectContextEdge{},
		&domain.TaskContextSnapshot{},
		&domain.AppSettings{},
		&domain.GitRepository{},
		&domain.GitWorktree{},
		&domain.GitBranch{},
		&SchemaMeta{},
	); err != nil {
		return fmt.Errorf("automigrate task models: %w", err)
	}
	if err := migrateRemoveSubtasks(ctx, db); err != nil {
		return fmt.Errorf("migrate remove subtasks: %w", err)
	}
	if err := migrateRemoveTaskType(ctx, db); err != nil {
		return fmt.Errorf("migrate remove task type: %w", err)
	}
	if err := migrateRemoveDraftEvaluations(ctx, db); err != nil {
		return fmt.Errorf("migrate remove draft evaluations: %w", err)
	}
	defaultProject := domain.DefaultProject(time.Now().UTC())
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&defaultProject).Error; err != nil {
		return fmt.Errorf("seed default project: %w", err)
	}
	if err := backfillLegacyChecklistCompletions(ctx, db); err != nil {
		return fmt.Errorf("backfill checklist completions: %w", err)
	}
	if err := migrateChecklistCheckToText(ctx, db); err != nil {
		return fmt.Errorf("migrate checklist check column: %w", err)
	}
	if err := migrateDropPromptAutomations(ctx, db); err != nil {
		return fmt.Errorf("migrate drop prompt automations: %w", err)
	}
	if err := dropLegacyGoalStepTables(ctx, db); err != nil {
		return fmt.Errorf("drop legacy goal/step tables: %w", err)
	}
	if err := migrateRepoRootToGitRepository(ctx, db); err != nil {
		return fmt.Errorf("migrate repo_root to git repository: %w", err)
	}
	if err := migrateDropRepoRootColumn(ctx, db); err != nil {
		return fmt.Errorf("drop app_settings.repo_root: %w", err)
	}
	if err := migrateSeedWorktreeBranchTree(ctx, db); err != nil {
		return fmt.Errorf("seed worktree-branch tree: %w", err)
	}
	if err := migrateContractGitTree(ctx, db); err != nil {
		return fmt.Errorf("contract git tree: %w", err)
	}
	if err := migrateFixedWorktreeBranch(ctx, db); err != nil {
		return fmt.Errorf("fixed worktree branch: %w", err)
	}
	if err := RecordSchemaRevision(ctx, db, time.Now().UTC()); err != nil {
		return fmt.Errorf("record schema revision: %w", err)
	}
	return nil
}

// dropLegacyGoalStepTables removes project_goals and project_steps after the
// flat task hierarchy migration. Idempotent — safe on fresh and upgraded DBs.
func dropLegacyGoalStepTables(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.dropLegacyGoalStepTables")
	if db.Dialector != nil && db.Dialector.Name() == "postgres" {
		if err := db.WithContext(ctx).Exec(`DROP TABLE IF EXISTS project_steps CASCADE`).Error; err != nil {
			return fmt.Errorf("drop project_steps: %w", err)
		}
		if err := db.WithContext(ctx).Exec(`DROP TABLE IF EXISTS project_goals CASCADE`).Error; err != nil {
			return fmt.Errorf("drop project_goals: %w", err)
		}
		if err := db.WithContext(ctx).Exec(`ALTER TABLE tasks DROP COLUMN IF EXISTS project_step_id`).Error; err != nil {
			return fmt.Errorf("drop tasks.project_step_id: %w", err)
		}
		return nil
	}
	if err := db.WithContext(ctx).Exec(`DROP TABLE IF EXISTS project_steps`).Error; err != nil {
		return fmt.Errorf("drop project_steps: %w", err)
	}
	if err := db.WithContext(ctx).Exec(`DROP TABLE IF EXISTS project_goals`).Error; err != nil {
		return fmt.Errorf("drop project_goals: %w", err)
	}
	return nil
}

// backfillLegacyChecklistCompletions marks pre-V1.1 completion rows so
// ValidateCanMarkDoneInTx continues to accept them after evidence columns ship.
func backfillLegacyChecklistCompletions(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillLegacyChecklistCompletions")
	res := db.WithContext(ctx).Exec(`
UPDATE task_checklist_completions
   SET verified_by = ?
 WHERE (verified_by IS NULL OR verified_by = '')
   AND (evidence IS NULL OR evidence = '')`,
		string(domain.VerifierLegacy),
	)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

// migrateChecklistCheckToText merges legacy shell-check commands into criterion
// text, then drops the check column and app_settings.check_command_timeout_seconds.
// Postgres only; SQLite test DBs rely on AutoMigrate after the domain field removal.
// Idempotent: skips the merge when the column was already dropped on a prior boot.
func migrateChecklistCheckToText(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateChecklistCheckToText")
	if db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	hasCheck, err := postgresTableHasColumn(ctx, db, "task_checklist_items", "check")
	if err != nil {
		return fmt.Errorf("probe task_checklist_items.check: %w", err)
	}
	if hasCheck {
		if err := db.WithContext(ctx).Exec(`
UPDATE task_checklist_items
   SET text = text || ' (verification: ' || trim("check") || ')'
 WHERE trim("check") != ''
   AND text NOT LIKE '%(verification:%'`).Error; err != nil {
			return fmt.Errorf("merge checklist check into text: %w", err)
		}
		if err := db.WithContext(ctx).Exec(`ALTER TABLE task_checklist_items DROP COLUMN IF EXISTS "check"`).Error; err != nil {
			return fmt.Errorf("drop task_checklist_items.check: %w", err)
		}
	}
	if err := db.WithContext(ctx).Exec(`ALTER TABLE app_settings DROP CONSTRAINT IF EXISTS chk_app_settings_check_timeout`).Error; err != nil {
		return fmt.Errorf("drop app_settings check timeout constraint: %w", err)
	}
	if err := db.WithContext(ctx).Exec(`ALTER TABLE app_settings DROP COLUMN IF EXISTS check_command_timeout_seconds`).Error; err != nil {
		return fmt.Errorf("drop app_settings.check_command_timeout_seconds: %w", err)
	}
	return nil
}

// migrateDropPromptAutomations removes the prompt-automations feature schema.
// Postgres only; SQLite test DBs rely on AutoMigrate after domain field removal.
// Idempotent: safe on fresh and upgraded DBs.
func migrateDropPromptAutomations(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateDropPromptAutomations")
	if db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := db.WithContext(ctx).Exec(`ALTER TABLE tasks DROP COLUMN IF EXISTS automation_selections`).Error; err != nil {
		return fmt.Errorf("drop tasks.automation_selections: %w", err)
	}
	if err := db.WithContext(ctx).Exec(`DROP TABLE IF EXISTS automations`).Error; err != nil {
		return fmt.Errorf("drop automations table: %w", err)
	}
	return nil
}

func postgresTableHasColumn(ctx context.Context, db *gorm.DB, table, column string) (bool, error) {
	slog.Debug("trace", "operation", "postgres.postgresTableHasColumn", "table", table, "column", column)
	var n int64
	err := db.WithContext(ctx).Raw(`
SELECT COUNT(*) FROM information_schema.columns
 WHERE table_schema = CURRENT_SCHEMA()
   AND table_name = ?
   AND column_name = ?`, table, column).Scan(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// tableHasColumnPortable checks for a column using GORM's Migrator, works
// with both Postgres and SQLite. Used by backfill migrations that must
// become no-ops after contract columns are dropped.
//
//funclogmeasure:skip category=hot-path reason="Schema introspection helper; called at boot in Migrate."
func tableHasColumnPortable(db *gorm.DB, table, column string) bool {
	return db.Migrator().HasColumn(table, column)
}

var errEmptyDSN = errors.New("database DSN is empty")

// ensureQueryExecModeSimpleProtocol appends pgx's default_query_exec_mode when
// absent. Without this, ALTER TABLE / AutoMigrate can change the row type of
// SELECT * while pooled connections still hold cached prepared statements,
// producing PostgreSQL error 0A000 "cached plan must not change result type".
// Simple protocol avoids server-side plan caching for that failure mode.
//
// See pgx ParseConfig: default_query_exec_mode=simple_protocol.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ensureQueryExecModeSimpleProtocol(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return dsn
	}
	if strings.Contains(dsn, "default_query_exec_mode=") {
		return dsn
	}
	const param = "default_query_exec_mode=simple_protocol"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if strings.Contains(dsn, "?") {
			return dsn + "&" + param
		}
		return dsn + "?" + param
	}
	return dsn + " " + param
}
