# Observability / coverage measurement for the whole Go module.
# Runs tests with a merged cover profile for ./... (every package, including cmd/*).
# go tool cover -func lists only production .go files (not *_test.go) that appear in the profile.
# Resolves the repo root from this script's path (cwd does not matter).
# Usage: .\scripts\measure-observability.ps1
# Writes coverage-observability.out under the repo root (gitignored via coverage*.out).
$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo
Write-Host "repo: $repo"

$prof = Join-Path $repo "coverage-observability.out"
go test ./... -coverprofile=$prof -count=1

# Capture stdout only (avoid 2>&1 here: stderr as ErrorRecord breaks Select-String).
$coverTmp = Join-Path ([IO.Path]::GetTempPath()) "t2a-cover-func-$PID.txt"
go tool cover "-func=$prof" | Set-Content -LiteralPath $coverTmp -Encoding utf8
$coverFunc = Get-Content -LiteralPath $coverTmp
Remove-Item -LiteralPath $coverTmp -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Per-function slog presence (static check, not coverage): go run ./cmd/funclogmeasure  (or scripts/measure-func-slog.*)"
Write-Host ""
Write-Host "=== Per-function test coverage (all production .go files in this profile) ==="
$coverFunc

Write-Host ""
Write-Host "Profile: $prof (gitignored)"
