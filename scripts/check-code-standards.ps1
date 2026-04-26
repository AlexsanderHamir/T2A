# CODE_STANDARDS.mdc guardrails (see .cursor/rules/CODE_STANDARDS.mdc Part 8).
# Exit 0 when clean; exit 1 when a rule is violated.
# Run from repository root: pwsh -File scripts/check-code-standards.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path (Join-Path $root "go.mod"))) {
    Write-Error "Run from repository root (go.mod not found next to scripts/)."
    exit 1
}

$failed = $false

function Test-IsUnderWebSrcApi {
    param([string]$FullPath)
    # CI (Linux) uses `/`; Windows uses `\`. `-like '*\web\src\api\*'` only matched Windows.
    $n = $FullPath.Replace('\', '/')
    return $n.Contains('/web/src/api/')
}

# TypeScript: fetch() must only appear under web/src/api/ (exclude tests).
$srcRoot = Join-Path $root (Join-Path "web" "src")
if (Test-Path $srcRoot) {
    $tsFiles = Get-ChildItem -Path $srcRoot -Recurse -Include *.ts, *.tsx -File |
        Where-Object {
            (-not (Test-IsUnderWebSrcApi $_.FullName)) -and
            $_.Name -notmatch '\.test\.(ts|tsx)$' -and
            ($_.FullName.Replace('\', '/') -notmatch '/test/')
        }
    # Match global `fetch(` only: exclude `.refetch(` / `prefetch(` and JSDoc
    # lines like "stats fetch (" where `fetch` is not the global.
    $fetchPat = '(?:^|[^\w.])fetch\s*\('
    foreach ($f in $tsFiles) {
        $text = Get-Content -LiteralPath $f.FullName -Raw
        if ($null -eq $text) { continue }
        if ($text -match $fetchPat) {
            Write-Host "VIOLATION: fetch( outside web/src/api/: $($f.FullName)" -ForegroundColor Red
            $failed = $true
        }
    }
}

$stylesRoot = Join-Path $srcRoot (Join-Path "app" "styles")
if (Test-Path $stylesRoot) {
    $cssFiles = Get-ChildItem -Path $stylesRoot -Recurse -Filter *.css -File
    $componentCssFiles = $cssFiles | Where-Object {
        $_.FullName.Replace('\', '/') -notmatch '/web/src/app/styles/tokens/'
    }
    $rawColorPat = '#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\('
    foreach ($f in $componentCssFiles) {
        $text = Get-Content -LiteralPath $f.FullName -Raw
        if ($null -eq $text) { continue }
        if ($text -match $rawColorPat) {
            Write-Host "VIOLATION: raw color outside web style tokens: $($f.FullName)" -ForegroundColor Red
            $failed = $true
        }
    }

    $tooSmallRemPat = 'font-size:\s*0\.[0-6][0-9]*rem'
    foreach ($f in $componentCssFiles) {
        $text = Get-Content -LiteralPath $f.FullName -Raw
        if ($null -eq $text) { continue }
        if ($text -match $tooSmallRemPat) {
            Write-Host "VIOLATION: font-size below --text-xs in component CSS: $($f.FullName)" -ForegroundColor Red
            $failed = $true
        }
    }
}

# Note: pkgs/tasks/domain embeds GORM struct tags and gorm.io/datatypes; a
# naive "no gorm in domain" check would false-positive. Tightening domain
# purity is a later CODE_STANDARDS stage (split models vs pure domain).

# Go: handler must not import database drivers directly.
$handlerRoot = Join-Path $root (Join-Path "pkgs" (Join-Path "tasks" "handler"))
if (Test-Path $handlerRoot) {
    $goFiles = Get-ChildItem -Path $handlerRoot -Recurse -Filter *.go -File |
        Where-Object { $_.Name -notmatch '_test\.go$' }
    foreach ($f in $goFiles) {
        $text = Get-Content -LiteralPath $f.FullName -Raw
        if ($text -match 'database/sql|jackc/pgx|gorm\.io/gorm') {
            Write-Host "VIOLATION: handler imports DB stack: $($f.FullName)" -ForegroundColor Red
            $failed = $true
        }
    }
}

if ($failed) {
    exit 1
}
Write-Host "check-code-standards: OK" -ForegroundColor Green
exit 0
