package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func newSettingsStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	return NewStore(tasktestdb.OpenSQLite(t)), context.Background()
}

func ptrString(v string) *string { return &v }
func ptrInt(v int) *int          { return &v }

// TestStore_GetSettings_seedsDefaultsOnFirstRead pins the contract that
// a fresh DB returns a fully populated AppSettings (runner=cursor, no
// repo root, no cursor bin override, no run-duration cap). The handler
// relies on this invariant: GET /settings never returns a sparse row.
func TestStore_GetSettings_seedsDefaultsOnFirstRead(t *testing.T) {
	s, ctx := newSettingsStore(t)
	got, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	want := domain.DefaultAppSettings()
	if got.Runner != want.Runner {
		t.Errorf("Runner = %q, want %q", got.Runner, want.Runner)
	}
	if got.CursorBin != want.CursorBin {
		t.Errorf("CursorBin = %q, want %q", got.CursorBin, want.CursorBin)
	}
	if got.MaxRunDurationSeconds != want.MaxRunDurationSeconds {
		t.Errorf("MaxRunDurationSeconds = %d, want %d", got.MaxRunDurationSeconds, want.MaxRunDurationSeconds)
	}
	if got.AgentPickupDelaySeconds != want.AgentPickupDelaySeconds {
		t.Errorf("AgentPickupDelaySeconds = %d, want %d", got.AgentPickupDelaySeconds, want.AgentPickupDelaySeconds)
	}
	if got.DisplayTimezone != want.DisplayTimezone {
		t.Errorf("DisplayTimezone = %q, want %q", got.DisplayTimezone, want.DisplayTimezone)
	}
	if got.DisplayTimezone != "" {
		t.Errorf("DisplayTimezone default = %q, want empty (the auto-detect sentinel — see domain.DefaultDisplayTimezone)", got.DisplayTimezone)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after seed")
	}
}

// TestStore_GetSettings_isIdempotent verifies that calling Get twice
// against an empty DB doesn't create two rows (the singleton invariant)
// and returns the same row both times.
func TestStore_GetSettings_isIdempotent(t *testing.T) {
	s, ctx := newSettingsStore(t)
	first, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	second, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("row id changed: %d -> %d", first.ID, second.ID)
	}
	if first.UpdatedAt != second.UpdatedAt {
		t.Errorf("UpdatedAt drifted: %v -> %v (Get must not mutate)", first.UpdatedAt, second.UpdatedAt)
	}
}

// TestStore_UpdateSettings_partialPatchRoundtrip pins PATCH semantics:
// only non-nil fields are overlaid, all other fields preserve their
// prior value. This is the contract PATCH /settings exposes to the SPA.
func TestStore_UpdateSettings_partialPatchRoundtrip(t *testing.T) {
	s, ctx := newSettingsStore(t)
	if _, err := s.GetSettings(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := s.UpdateSettings(ctx, SettingsPatch{
		CursorBin:             ptrString("/tmp/cursor"),
		MaxRunDurationSeconds: ptrInt(900),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CursorBin != "/tmp/cursor" {
		t.Errorf("CursorBin = %q, want /tmp/cursor", updated.CursorBin)
	}
	if updated.MaxRunDurationSeconds != 900 {
		t.Errorf("MaxRunDurationSeconds = %d, want 900", updated.MaxRunDurationSeconds)
	}
	if updated.Runner != "cursor" {
		t.Errorf("Runner = %q, want cursor (not in patch)", updated.Runner)
	}

	persisted, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("re-get: %v", err)
	}
	if persisted.CursorBin != updated.CursorBin {
		t.Errorf("CursorBin did not persist: got %q, want %q", persisted.CursorBin, updated.CursorBin)
	}
	if persisted.MaxRunDurationSeconds != updated.MaxRunDurationSeconds {
		t.Errorf("MaxRunDurationSeconds did not persist: got %d, want %d", persisted.MaxRunDurationSeconds, updated.MaxRunDurationSeconds)
	}
}

// TestStore_UpdateSettings_createsRowOnFirstWrite covers the case where
// PATCH lands before GET ever ran. The transaction must seed the row
// from defaults and then overlay the patch.
func TestStore_UpdateSettings_createsRowOnFirstWrite(t *testing.T) {
	s, ctx := newSettingsStore(t)
	updated, err := s.UpdateSettings(ctx, SettingsPatch{
		CursorBin: ptrString("/srv/cursor"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CursorBin != "/srv/cursor" {
		t.Errorf("CursorBin = %q, want /srv/cursor", updated.CursorBin)
	}
	if updated.Runner != "cursor" {
		t.Errorf("Runner = %q, want cursor (default seed)", updated.Runner)
	}
}

// TestStore_UpdateSettings_trimsAndClamps pins the input-hygiene rules:
// trimming whitespace on string fields and rejecting empty Runner /
// negative MaxRunDurationSeconds with ErrInvalidInput so the handler
// can map them to 400.
func TestStore_UpdateSettings_trimsAndClamps(t *testing.T) {
	t.Run("trims_strings", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		updated, err := s.UpdateSettings(ctx, SettingsPatch{
			CursorBin: ptrString("\tcursor\n"),
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.CursorBin != "cursor" {
			t.Errorf("CursorBin = %q, want cursor (whitespace trimmed)", updated.CursorBin)
		}
	})

	t.Run("rejects_empty_runner", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		_, err := s.UpdateSettings(ctx, SettingsPatch{Runner: ptrString("   ")})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects_negative_max_duration", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		_, err := s.UpdateSettings(ctx, SettingsPatch{MaxRunDurationSeconds: ptrInt(-1)})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects_invalid_agent_pickup_delay", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		_, err := s.UpdateSettings(ctx, SettingsPatch{AgentPickupDelaySeconds: ptrInt(-1)})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
		_, err = s.UpdateSettings(ctx, SettingsPatch{AgentPickupDelaySeconds: ptrInt(604801)})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("accepts_zero_max_duration", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		updated, err := s.UpdateSettings(ctx, SettingsPatch{MaxRunDurationSeconds: ptrInt(0)})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.MaxRunDurationSeconds != 0 {
			t.Errorf("MaxRunDurationSeconds = %d, want 0 (no limit)", updated.MaxRunDurationSeconds)
		}
	})

	t.Run("accepts_valid_iana_timezone", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		updated, err := s.UpdateSettings(ctx, SettingsPatch{DisplayTimezone: ptrString("America/New_York")})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.DisplayTimezone != "America/New_York" {
			t.Errorf("DisplayTimezone = %q, want America/New_York", updated.DisplayTimezone)
		}
		got, err := s.GetSettings(ctx)
		if err != nil {
			t.Fatalf("re-get: %v", err)
		}
		if got.DisplayTimezone != "America/New_York" {
			t.Errorf("DisplayTimezone did not persist: got %q", got.DisplayTimezone)
		}
	})

	t.Run("rejects_invalid_iana_timezone", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		_, err := s.UpdateSettings(ctx, SettingsPatch{DisplayTimezone: ptrString("Not/A_Real_Zone")})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("accepts_empty_timezone_as_auto_detect_reset", func(t *testing.T) {
		// Empty string is the documented "clear override, fall back to
		// browser auto-detect" sentinel (see domain.DefaultDisplayTimezone).
		// Whitespace-only is treated the same after TrimSpace. Both must
		// round-trip through the store without an error so operators can
		// reset an explicit zone back to auto-detect via PATCH /settings
		// { "display_timezone": "" }.
		s, ctx := newSettingsStore(t)
		if _, err := s.UpdateSettings(ctx, SettingsPatch{DisplayTimezone: ptrString("Europe/London")}); err != nil {
			t.Fatalf("seed explicit zone: %v", err)
		}
		updated, err := s.UpdateSettings(ctx, SettingsPatch{DisplayTimezone: ptrString("   ")})
		if err != nil {
			t.Fatalf("clear to empty: %v", err)
		}
		if updated.DisplayTimezone != "" {
			t.Errorf("DisplayTimezone = %q, want \"\" (auto-detect sentinel)", updated.DisplayTimezone)
		}
	})

	t.Run("trims_timezone", func(t *testing.T) {
		s, ctx := newSettingsStore(t)
		updated, err := s.UpdateSettings(ctx, SettingsPatch{DisplayTimezone: ptrString("  Europe/London  ")})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.DisplayTimezone != "Europe/London" {
			t.Errorf("DisplayTimezone = %q, want Europe/London (trimmed)", updated.DisplayTimezone)
		}
	})
}

// TestStore_UpdateSettings_concurrentPatchesPreserveSingleton hammers
// the row with parallel writers to confirm the SELECT ... FOR UPDATE
// path serializes them correctly. Final state must match exactly one of
// the writers; the row count must be 1 (the singleton invariant must
// not be broken under contention).
func TestStore_UpdateSettings_concurrentPatchesPreserveSingleton(t *testing.T) {
	s, ctx := newSettingsStore(t)
	if _, err := s.GetSettings(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const writers = 8
	var wg sync.WaitGroup
	wg.Add(writers)
	errCh := make(chan error, writers)
	for i := 0; i < writers; i++ {
		i := i
		go func() {
			defer wg.Done()
			val := i
			if _, err := s.UpdateSettings(ctx, SettingsPatch{MaxRunDurationSeconds: ptrInt(val)}); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent update: %v", err)
		}
	}

	final, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("final get: %v", err)
	}
	if final.MaxRunDurationSeconds < 0 || final.MaxRunDurationSeconds >= writers {
		t.Errorf("MaxRunDurationSeconds = %d, want one of 0..%d", final.MaxRunDurationSeconds, writers-1)
	}

	var rowCount int64
	if err := s.db.Table("app_settings").Count(&rowCount).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("row count = %d, want 1 (singleton invariant)", rowCount)
	}
}

// TestSettingsPatch_IsEmpty pins the helper used by the handler to
// short-circuit no-op PATCH requests.
func TestSettingsPatch_IsEmpty(t *testing.T) {
	if !(SettingsPatch{}).IsEmpty() {
		t.Error("zero-value patch should report IsEmpty() == true")
	}
	if (SettingsPatch{CursorBin: ptrString("")}).IsEmpty() {
		t.Error("patch with one explicit field should report IsEmpty() == false")
	}
}

// TestStore_UpdateSettings_dualWriteCursorToRunnerConfigs verifies that
// patching CursorBin or CursorModel also updates the runner_configs
// column with the corresponding cursor config blob.
func TestStore_UpdateSettings_dualWriteCursorToRunnerConfigs(t *testing.T) {
	s, ctx := newSettingsStore(t)
	if _, err := s.GetSettings(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := s.UpdateSettings(ctx, SettingsPatch{
		CursorBin:   ptrString("/usr/local/bin/cursor-agent"),
		CursorModel: ptrString("opus-4.1"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	var configs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(updated.RunnerConfigs), &configs); err != nil {
		t.Fatalf("unmarshal runner_configs: %v (raw=%s)", err, updated.RunnerConfigs)
	}
	cursorRaw, ok := configs["cursor"]
	if !ok {
		t.Fatal("runner_configs missing 'cursor' key")
	}
	var cursorCfg map[string]string
	if err := json.Unmarshal(cursorRaw, &cursorCfg); err != nil {
		t.Fatalf("unmarshal cursor config: %v (raw=%s)", err, cursorRaw)
	}
	if cursorCfg["binary_path"] != "/usr/local/bin/cursor-agent" {
		t.Errorf("binary_path = %q, want /usr/local/bin/cursor-agent", cursorCfg["binary_path"])
	}
	if cursorCfg["default_model"] != "opus-4.1" {
		t.Errorf("default_model = %q, want opus-4.1", cursorCfg["default_model"])
	}

	persisted, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("re-get: %v", err)
	}
	var persistedConfigs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(persisted.RunnerConfigs), &persistedConfigs); err != nil {
		t.Fatalf("unmarshal persisted runner_configs: %v", err)
	}
	if _, ok := persistedConfigs["cursor"]; !ok {
		t.Error("persisted runner_configs missing 'cursor' key after re-read")
	}
}

// TestStore_UpdateSettings_dualWriteRunnerConfigsPatchField verifies that
// setting RunnerConfigs via the Patch field persists the blob.
func TestStore_UpdateSettings_dualWriteRunnerConfigsPatchField(t *testing.T) {
	s, ctx := newSettingsStore(t)
	if _, err := s.GetSettings(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	blob := json.RawMessage(`{"cursor":{"binary_path":"custom","default_model":"test"}}`)
	updated, err := s.UpdateSettings(ctx, SettingsPatch{
		RunnerConfigs: &blob,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	var configs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(updated.RunnerConfigs), &configs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := configs["cursor"]; !ok {
		t.Error("runner_configs missing 'cursor' key")
	}
}
