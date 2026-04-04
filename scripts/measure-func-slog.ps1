# Measures how many named functions/methods contain a direct log/slog call (see docs/OBSERVABILITY.md).
# Resolves repo root from this script path; extra args are passed to funclogmeasure (e.g. -json, -enforce).
$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo
go run ./cmd/funclogmeasure @args
