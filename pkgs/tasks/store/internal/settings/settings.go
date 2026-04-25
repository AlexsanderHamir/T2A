package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const logCmd = "taskapi"

// Patch is the partial-update payload for app_settings. Pointer-typed
// fields distinguish "not provided" (nil) from "set to zero value"
// (e.g. *int = 0 means MaxRunDurationSeconds explicitly set to 0 == "no
// limit"; nil means "leave unchanged").
type Patch struct {
	WorkerEnabled           *bool
	AgentPaused             *bool
	Runner                  *string
	RepoRoot                *string
	CursorBin               *string
	CursorModel             *string
	MaxRunDurationSeconds   *int
	AgentPickupDelaySeconds *int
	// DisplayTimezone is an IANA timezone identifier validated via
	// time.LoadLocation. Empty string ("") is accepted and means
	// "clear the override — let the SPA auto-detect the operator's
	// browser timezone" (the domain.DefaultDisplayTimezone sentinel).
	// Any non-empty value must parse via time.LoadLocation.
	DisplayTimezone *string
	// OptimisticMutationsEnabled / SSEReplayEnabled are realtime rollout flags.
	// See domain.AppSettings for the per-flag semantics.
	OptimisticMutationsEnabled *bool
	SSEReplayEnabled           *bool
}

// IsEmpty reports whether the patch has nothing to apply. Used by the
// HTTP handler to short-circuit no-op PATCH calls without a DB write.
// Skip-listed in cmd/funclogmeasure/analyze.go: pure five-pointer-nil
// predicate called once per PATCH /settings request, where the surrounding
// handler already logs the no-op short-circuit decision.
func (p Patch) IsEmpty() bool {
	return p.WorkerEnabled == nil &&
		p.AgentPaused == nil &&
		p.Runner == nil &&
		p.RepoRoot == nil &&
		p.CursorBin == nil &&
		p.CursorModel == nil &&
		p.MaxRunDurationSeconds == nil &&
		p.AgentPickupDelaySeconds == nil &&
		p.DisplayTimezone == nil &&
		p.OptimisticMutationsEnabled == nil &&
		p.SSEReplayEnabled == nil
}

// Get returns the singleton app_settings row, creating it with
// domain.DefaultAppSettings on first read so callers always observe a
// populated value. The create-on-read is guarded by the unique CHECK
// constraint on id=1: a parallel Get from another goroutine that wins
// the insert race will simply re-read the row the loser created.
func Get(ctx context.Context, db *gorm.DB) (domain.AppSettings, error) {
	defer kernel.DeferLatency(kernel.OpGetAppSettings)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.settings.Get")
	if db == nil {
		return domain.AppSettings{}, errors.New("tasks store: nil database")
	}
	var row domain.AppSettings
	err := db.WithContext(ctx).First(&row, "id = ?", domain.AppSettingsRowID).Error
	if err == nil {
		if !row.OptimisticMutationsEnabled || !row.SSEReplayEnabled {
			t := true
			return Update(ctx, db, Patch{
				OptimisticMutationsEnabled: &t,
				SSEReplayEnabled:           &t,
			})
		}
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AppSettings{}, fmt.Errorf("get app settings: %w", err)
	}
	seed := domain.DefaultAppSettings()
	seed.UpdatedAt = time.Now().UTC()
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return domain.AppSettings{}, fmt.Errorf("seed app settings: %w", err)
	}
	if err := db.WithContext(ctx).First(&row, "id = ?", domain.AppSettingsRowID).Error; err != nil {
		return domain.AppSettings{}, fmt.Errorf("get app settings (post-seed): %w", err)
	}
	slog.Info("app settings seeded with defaults",
		"cmd", logCmd, "operation", "tasks.store.settings.seeded",
		"worker_enabled", row.WorkerEnabled, "agent_paused", row.AgentPaused,
		"runner", row.Runner,
		"repo_root", row.RepoRoot, "cursor_bin", row.CursorBin,
		"max_run_duration_seconds", row.MaxRunDurationSeconds,
		"display_timezone", row.DisplayTimezone)
	return row, nil
}

// Update applies a partial Patch to the singleton row inside a
// transaction. If the row doesn't exist yet it is created from
// domain.DefaultAppSettings before the patch is overlaid, so the first
// PATCH against a fresh DB is well-defined.
//
// Validation enforced here:
//   - Runner: trimmed; if explicitly set to "" the call returns
//     domain.ErrInvalidInput (the runner must be a known id; the
//     handler is responsible for checking it against the registry, this
//     layer just refuses the empty-string degenerate case).
//   - MaxRunDurationSeconds: must be >= 0 (matches the DB CHECK).
//
// The caller (the handler / supervisor) owns higher-level validation
// such as "does this path exist on disk" and "is this runner id in the
// registry" — those depend on out-of-DB state and don't belong in the
// store layer.
func Update(ctx context.Context, db *gorm.DB, patch Patch) (domain.AppSettings, error) {
	defer kernel.DeferLatency(kernel.OpUpdateAppSettings)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.settings.Update")
	if db == nil {
		return domain.AppSettings{}, errors.New("tasks store: nil database")
	}
	if err := validatePatch(patch); err != nil {
		return domain.AppSettings{}, err
	}
	var out domain.AppSettings
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row domain.AppSettings
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", domain.AppSettingsRowID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = domain.DefaultAppSettings()
			row.UpdatedAt = time.Now().UTC()
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return fmt.Errorf("seed app settings during update: %w", err)
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", domain.AppSettingsRowID).Error; err != nil {
				return fmt.Errorf("re-read app settings after seed: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("lock app settings: %w", err)
		}
		applyPatch(&row, patch)
		row.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&row).Error; err != nil {
			return fmt.Errorf("save app settings: %w", err)
		}
		out = row
		return nil
	})
	if txErr != nil {
		return domain.AppSettings{}, txErr
	}
	return out, nil
}

func validatePatch(patch Patch) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.settings.validatePatch")
	if patch.Runner != nil {
		trimmed := strings.TrimSpace(*patch.Runner)
		if trimmed == "" {
			return fmt.Errorf("%w: runner must be non-empty", domain.ErrInvalidInput)
		}
	}
	if patch.MaxRunDurationSeconds != nil && *patch.MaxRunDurationSeconds < 0 {
		return fmt.Errorf("%w: max_run_duration_seconds must be >= 0", domain.ErrInvalidInput)
	}
	if patch.AgentPickupDelaySeconds != nil {
		v := *patch.AgentPickupDelaySeconds
		if v < 0 || v > 604800 {
			return fmt.Errorf("%w: agent_pickup_delay_seconds must be between 0 and 604800", domain.ErrInvalidInput)
		}
	}
	if patch.CursorModel != nil && len(strings.TrimSpace(*patch.CursorModel)) > 256 {
		return fmt.Errorf("%w: cursor_model too long (max 256)", domain.ErrInvalidInput)
	}
	if patch.DisplayTimezone != nil {
		trimmed := strings.TrimSpace(*patch.DisplayTimezone)
		// Empty string is allowed and clears the override so the SPA
		// falls back to browser auto-detect (see
		// domain.DefaultDisplayTimezone). Non-empty values must parse
		// as an IANA zone so a stale SPA PATCH can never poison the row
		// with garbage that would later crash Intl.DateTimeFormat.
		if trimmed != "" {
			if _, err := time.LoadLocation(trimmed); err != nil {
				return fmt.Errorf("%w: display_timezone %q is not a valid IANA timezone: %v", domain.ErrInvalidInput, trimmed, err)
			}
		}
	}
	return nil
}

func applyPatch(row *domain.AppSettings, patch Patch) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.settings.applyPatch")
	if patch.WorkerEnabled != nil {
		row.WorkerEnabled = *patch.WorkerEnabled
	}
	if patch.AgentPaused != nil {
		row.AgentPaused = *patch.AgentPaused
	}
	if patch.Runner != nil {
		row.Runner = strings.TrimSpace(*patch.Runner)
	}
	if patch.RepoRoot != nil {
		row.RepoRoot = strings.TrimSpace(*patch.RepoRoot)
	}
	if patch.CursorBin != nil {
		row.CursorBin = strings.TrimSpace(*patch.CursorBin)
	}
	if patch.CursorModel != nil {
		row.CursorModel = strings.TrimSpace(*patch.CursorModel)
	}
	if patch.MaxRunDurationSeconds != nil {
		row.MaxRunDurationSeconds = *patch.MaxRunDurationSeconds
	}
	if patch.AgentPickupDelaySeconds != nil {
		row.AgentPickupDelaySeconds = *patch.AgentPickupDelaySeconds
	}
	if patch.DisplayTimezone != nil {
		// validatePatch already confirmed the zone parses; store the
		// trimmed-but-unmodified string so the operator's choice round-trips
		// verbatim (LoadLocation is forgiving about case but the SPA selects
		// from Intl.supportedValuesOf which is canonical).
		row.DisplayTimezone = strings.TrimSpace(*patch.DisplayTimezone)
	}
	if patch.OptimisticMutationsEnabled != nil {
		row.OptimisticMutationsEnabled = *patch.OptimisticMutationsEnabled
	}
	if patch.SSEReplayEnabled != nil {
		row.SSEReplayEnabled = *patch.SSEReplayEnabled
	}
}
