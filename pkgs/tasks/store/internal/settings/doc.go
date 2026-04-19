// Package settings owns app_settings persistence — the singleton row
// (id=1) holding all UI-configurable runtime settings (agent worker
// enabled flag, runner choice, repo root, cursor binary path, max run
// duration). The public store facade re-exports the model and CRUD via
// (*Store).GetSettings / (*Store).UpdateSettings.
//
// Two architectural rules pinned by this package:
//
//  1. The DB row is the only source of truth. No env-var fallback, no
//     config-file fallback. The Get path auto-creates the row from
//     domain.DefaultAppSettings on first read so callers always get a
//     fully populated value.
//
//  2. Update is a partial PATCH: the caller supplies a Patch struct of
//     pointer-typed fields and only non-nil fields are overlaid. This
//     keeps PATCH /settings clients from accidentally clobbering fields
//     they didn't intend to touch.
package settings
