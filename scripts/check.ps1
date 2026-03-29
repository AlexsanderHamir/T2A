# Full local verification: gofmt (check), go vet, go test, web npm test + build.
# Usage from repo root: .\scripts\check.ps1
# Skip web steps: $env:CHECK_SKIP_WEB = "1"
$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repo

Write-Host "gofmt (check)..." -ForegroundColor Cyan
$gofmtOut = & gofmt -l .
if ($gofmtOut) {
    Write-Host "These files need gofmt:" -ForegroundColor Red
    $gofmtOut
    exit 1
}

Write-Host "go vet..." -ForegroundColor Cyan
& go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "go test..." -ForegroundColor Cyan
& go test ./... -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($env:CHECK_SKIP_WEB -eq "1") {
    Write-Host "check OK (web skipped)" -ForegroundColor Green
    exit 0
}

$webDir = Join-Path $repo "web"
if (-not (Test-Path (Join-Path $webDir "package.json"))) {
    Write-Host "check OK" -ForegroundColor Green
    exit 0
}

Write-Host "web: npm test..." -ForegroundColor Cyan
Push-Location $webDir
try {
    & npm test -- --run
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "web: npm run build..." -ForegroundColor Cyan
    & npm run build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}

Write-Host "check OK" -ForegroundColor Green
