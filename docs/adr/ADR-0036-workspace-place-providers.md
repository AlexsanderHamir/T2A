# ADR-0036: Workspace Place Providers

**Date:** 2026-06-23
**Status:** Superseded by [Cycle 7 browse policy cleanup](#cycle-7-amendment-2026-06-23)
**Deciders:** Engineering

## Context

The workspace folder picker lists directories the operator may register as a git repository. Default roots were install checkout + `$HOME`. On Windows with OneDrive folder backup, `$HOME/Documents` is not the same path Explorer shows — the shell known-folder API resolves to `OneDrive\Documents` instead.

An interim fix remapped English directory names (`Documents`, `Desktop`, …) after listing `$HOME`. That approach failed on localized Windows installs, missed folders redirected outside home, and duplicated logic the OS already provides.

## Decision

Introduce a **Place / PlaceProvider** abstraction in `pkgs/repo`:

- A **Place** is a labeled absolute directory with a stable **category** (`install`, `home`, `documents`, …).
- A **PlaceProvider** returns zero or more Places for the current environment.
- A **PlaceRegistry** composes providers in registration order and **dedupes by canonical path** (via `filepath.EvalSymlinks`).

Default registry order:

1. `InstallPlaceProvider` — Hamix checkout (`go.mod` root or `/app` in Docker).
2. `HomePlaceProvider` — operator home (`$HOME` or `/host-home` in Docker).
3. `UserDirsPlaceProvider` — OS-resolved profile folders (skipped in Docker).

Directory listing (`ListBrowseDirs` / `readBrowseSubdirs`) performs **pure I/O** — no name-based remapping.

### OS provider matrix

| OS | User-dirs source |
| --- | --- |
| Windows | `golang.org/x/sys/windows.KnownFolderPath` (`FOLDERID_Documents`, etc.) |
| macOS | `$HOME/Documents`, `$HOME/Desktop`, … (iCloud symlinks resolved by existing containment canonicalization) |
| Linux | `~/.config/user-dirs.dirs` (XDG); falls back to `$HOME/Documents` etc. when missing |
| Other | no user-dir Places |

### API

`GET /settings/workspace-roots` returns each root with an optional **`category`** field. The SPA groups roots into **Workspace** (`install`, `home`, `custom`) and **User folders** (`documents`, `desktop`, …).

`HAMIX_BROWSE_ROOTS` replaces the entire default registry with `CustomPlaceProvider` — user-dir providers are not included.

## Consequences

### Positive

- Picker matches native file managers on Windows + OneDrive, Linux XDG redirects, and typical macOS layouts.
- Localized on-disk folder names no longer matter — labels come from category, not basename.
- New picker locations (recents, bookmarks) add a provider without touching listing code.
- Per-OS code is isolated behind build tags.

### Negative / Trade-offs

- More files than the interim name-switch (~400 LOC net with tests).
- Linux relies on parsing `user-dirs.dirs` rather than invoking `xdg-user-dir` (no subprocess; format is stable).
- macOS does not call `NSFileManager` — assumes `$HOME/Documents` remains the canonical entry point (true for iCloud Desktop & Documents).

## Alternatives Considered

| Alternative | Reason Rejected |
| --- | --- |
| Name switch after `ReadDir` under `$HOME` | English-only, misses redirects outside home, conflates listing with resolution |
| Rename endpoint to `/settings/places` | Unnecessary breaking change; `category` extends existing shape |
| macOS cgo / `NSFileManager` | `$HOME` paths are symlinked by the OS; cgo adds build complexity for marginal gain |
| Inject user dirs as synthetic children of Home | Hides first-class folders; breadcrumb parent semantics become ambiguous |

---

## Cycle 7 Amendment (2026-06-23)

**Status:** This decision is superseded for `GET /settings/workspace-roots` behavior. The Place / PlaceProvider abstraction is retained but the **default registry is emptied**.

### What changed

`GET /settings/workspace-roots` now returns registered git repositories from the `git_repositories` table instead of OS-resolved place providers.

- `InstallPlaceProvider`, `HomePlaceProvider`, and `UserDirsPlaceProvider` are **retired from `defaultPlaceRegistry()`**. Their code is preserved for future reuse but no longer registered by default.
- `CustomPlaceProvider` (`HAMIX_BROWSE_ROOTS`) remains as an **ops override**: when set, it replaces DB-sourced roots entirely (for CI and restricted deployments).
- `GET /settings/workspace-roots` returns roots with `category: "registered"` for DB-sourced entries.
- `GET /settings/browse-dirs` switches to **full-disk (unrestricted) listing** when `HAMIX_BROWSE_ROOTS` is not set, enabling the register-repo bootstrap flow without requiring pre-configured roots.
- A new `repo.ListBrowseDirsUnrestricted` function handles the unrestricted case without path containment enforcement.
- A new `PlaceCategoryRegistered = "registered"` constant is added to `pkgs/repo`.

### Rationale

Once the git_repositories table is populated via the Register Repo flow (Cycles 5–6), the OS folder providers no longer serve a purpose as workspace roots — they suggest locations the user hasn't registered. The picker should reflect what has been deliberately registered, not what happens to exist on disk.

Full-disk browse is preserved in `browse-dirs` so operators can still navigate to unregistered repositories during the bootstrap registration flow.
