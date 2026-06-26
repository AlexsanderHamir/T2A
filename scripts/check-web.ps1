# Hamix web verification — source of truth for the CI web job.
#
# Steps: npm ci (-Install), web test, web lint, web standards, web build
#
# Usage (repo root): .\scripts\check-web.ps1 [flags]
#
# Flags:
#   -Verbose           Stream full tool output (CI uses this)
#   -Install           Run npm ci in web/ before other steps
#   -Group <name>      Restrict to lint|build|test-unit|test-components|test-app|test-task-pages|test-task-create|test-settings|test-projects|test-worktrees (CI matrix)
#   -Help              Show options
#
# CI:
#   ./scripts/check-web.sh --install --verbose --group=lint

param(
    [switch]$Help,
    [switch]$Verbose,
    [switch]$Install,
    [ValidateSet("lint", "build", "test-unit", "test-components", "test-app", "test-task-pages", "test-task-create", "test-settings", "test-projects", "test-worktrees", "")]
    [string]$Group = ""
)

if ($Help -or $args -contains '--help' -or $args -contains '-h') {
    Get-Content $PSCommandPath | Select-Object -Skip 1 -First 16 | ForEach-Object { $_ -replace '^# ?', '' }
    exit 0
}

$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repo

$webDir = Join-Path $repo "web"
if (-not (Test-Path (Join-Path $webDir "package.json"))) {
    Write-Error "web/package.json not found"
    exit 1
}

$CheckStart = Get-Date
$script:Step = 0
$script:Passed = 0

function Get-TotalSteps {
    param([string]$Scope)
    $base = switch ($Scope) {
        "lint" { 3 }
        "build" { 1 }
        { $_ -in "test-unit", "test-components", "test-app", "test-task-pages", "test-task-create", "test-settings", "test-projects", "test-worktrees" } { 1 }
        default { 12 }
    }
    if ($Install) { return $base + 1 }
    return $base
}

$script:Total = Get-TotalSteps $Group

function Format-Duration {
    param([TimeSpan]$Span)
    $secs = [int][Math]::Round($Span.TotalSeconds)
    if ($secs -lt 60) { return "${secs}s" }
    return "{0}m{1:D2}s" -f [Math]::Floor($secs / 60), ($secs % 60)
}

function Write-StepPrefix {
    $script:Step++
    Write-Host -NoNewline "[$($script:Step)/$($script:Total)] "
}

function Fail-Step {
    param(
        [string]$Name,
        [int]$Code = 1
    )
    Write-Host ""
    Write-Host "check FAILED: $Name ($($script:Step)/$($script:Total))" -ForegroundColor Red
    exit $Code
}

function Complete-Ok {
    $elapsed = (Get-Date) - $CheckStart
    Write-Host ""
    Write-Host "check OK  $($script:Passed)/$($script:Total) passed  $(Format-Duration $elapsed)" -ForegroundColor Green
    exit 0
}

function Write-OkLine {
    param(
        [string]$Label,
        [TimeSpan]$Elapsed,
        [string]$Stats = ""
    )
    $pad = [Math]::Max(1, 22 - $Label.Length)
    $line = (" " * $pad) + "ok $(Format-Duration $Elapsed)"
    if ($Stats) { $line += "  ($Stats)" }
    Write-Host $line -ForegroundColor Green
}

function Invoke-CapturedStep {
    param(
        [string]$Label,
        [scriptblock]$Command,
        [scriptblock]$StatsParser = $null
    )
    Write-StepPrefix
    Write-Host -NoNewline "$Label "

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $log = [System.IO.Path]::GetTempFileName()
    $code = 0
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'

    try {
        if ($Verbose) {
            Write-Host "..." -ForegroundColor Cyan
            & $Command
            $code = $LASTEXITCODE
        } else {
            & $Command 2>&1 | Out-File -FilePath $log -Encoding utf8
            $code = $LASTEXITCODE
        }
        if ($null -eq $code) { $code = 0 }
    } catch {
        $code = 1
        if (-not $Verbose) { $_ | Out-File -FilePath $log -Encoding utf8 -Append }
    } finally {
        $ErrorActionPreference = $prevEap
        $sw.Stop()
    }

    if ($code -eq 0) {
        $stats = ""
        if ($StatsParser -and -not $Verbose -and (Test-Path $log)) {
            $stats = & $StatsParser $log
        }
        $script:Passed++
        Write-OkLine $Label $sw.Elapsed $stats
        Remove-Item $log -Force -ErrorAction SilentlyContinue
        return
    }

    Write-Host "FAILED" -ForegroundColor Red
    if (-not $Verbose -and (Test-Path $log)) { Get-Content $log }
    Remove-Item $log -Force -ErrorAction SilentlyContinue
    Fail-Step $Label $code
}

function Get-WebTestStats {
    param([string]$LogPath)
    $content = Get-Content $LogPath -Raw
    $m = [regex]::Match($content, 'Tests\s+(\d+)\s+passed')
    if ($m.Success) { return "tests $($m.Groups[1].Value) passed" }
    return ""
}

function Get-WebLintStats {
    param([string]$LogPath)
    $content = Get-Content $LogPath -Raw
    $m = [regex]::Match($content, '(\d+)\s+warnings')
    if ($m.Success -and [int]$m.Groups[1].Value -gt 0) {
        return "$($m.Groups[1].Value) warnings"
    }
    return ""
}

function Invoke-WebTest {
    param(
        [string]$Label,
        [string[]]$ExtraArgs
    )
    Invoke-CapturedStep $Label {
        if ($Verbose) {
            & npm test -- --run @ExtraArgs
        } else {
            & npm test -- --run @ExtraArgs --reporter=basic
        }
    } { param($p) Get-WebTestStats $p }
}

function Invoke-MaybeNpmCi {
    if ($Install) {
        Invoke-CapturedStep "npm ci" { Push-Location $webDir; try { npm ci } finally { Pop-Location } }
    }
}

Write-Host "Hamix check (web)"
Write-Host ""

switch ($Group) {
    "lint" {
        Invoke-CapturedStep "check-brand" { & "$PSScriptRoot\check-brand.ps1" }
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-CapturedStep "web (lint)" { npm run lint } { param($p) Get-WebLintStats $p }
            Invoke-CapturedStep "web standards" { npm run check:standards }
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "build" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-CapturedStep "web (build)" { npm run build }
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-unit" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-unit)" @("--project=unit")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-components" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-components)" @("--project=components")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-app" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-app)" @("--project=app")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-task-pages" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-task-pages)" @("--project=task-pages")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-task-create" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-task-create)" @("--project=task-create")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-settings" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-settings)" @("--project=settings")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-projects" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-projects)" @("--project=projects")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
    "test-worktrees" {
        Invoke-MaybeNpmCi
        Push-Location $webDir
        try {
            Invoke-WebTest "web (test-worktrees)" @("--project=worktrees")
        } finally {
            Pop-Location
        }
        Complete-Ok
    }
}

Invoke-CapturedStep "check-brand" { & "$PSScriptRoot\check-brand.ps1" }
Invoke-MaybeNpmCi

Push-Location $webDir
try {
    Invoke-WebTest "web (test-unit)"        @("--project=unit")
    Invoke-WebTest "web (test-components)"  @("--project=components")
    Invoke-WebTest "web (test-app)"         @("--project=app")
    Invoke-WebTest "web (test-task-pages)"  @("--project=task-pages")
    Invoke-WebTest "web (test-task-create)" @("--project=task-create")
    Invoke-WebTest "web (test-settings)"    @("--project=settings")
    Invoke-WebTest "web (test-projects)"    @("--project=projects")
    Invoke-WebTest "web (test-worktrees)"   @("--project=worktrees")
    Invoke-CapturedStep "web (lint)" { npm run lint } { param($p) Get-WebLintStats $p }
    Invoke-CapturedStep "web standards" { npm run check:standards }
    Invoke-CapturedStep "web (build)" { npm run build }
} finally {
    Pop-Location
}

Complete-Ok
