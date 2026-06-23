# Hamix Go verification — source of truth for the CI backend job.
#
# Steps: gofmt, go vet, scheduling boundary, go test, funclogmeasure
#
# Usage (repo root): .\scripts\check-go.ps1 [flags]
#
# Flags:
#   -Verbose            Stream full tool output (CI uses this)
#   -SkipFunclog        Skip funclogmeasure -enforce
#   -LintOnly           Lint steps only (includes test-group coverage guard)
#   -TestsOnly          go test only (use with -Group for CI matrix cells)
#   -Group <name>       Restrict go test to core|tasks|agents|harness
#   -Help               Show options
#
# CI: ./scripts/check-go.sh --lint-only --verbose
#     ./scripts/check-go.sh --tests-only --group=core --verbose

param(
    [switch]$Help,
    [switch]$Verbose,
    [switch]$SkipFunclog,
    [switch]$LintOnly,
    [switch]$TestsOnly,
    [string]$Group = ""
)

if ($Help -or $args -contains '--help' -or $args -contains '-h') {
    Get-Content $PSCommandPath | Select-Object -Skip 1 -First 17 | ForEach-Object { $_ -replace '^# ?', '' }
    exit 0
}

if ($LintOnly -and $TestsOnly) {
    Write-Error "cannot use -LintOnly and -TestsOnly together"
    exit 2
}

$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repo

. (Join-Path $PSScriptRoot "test-groups.ps1")

if ($Group) {
    $null = Get-GroupPackages $Group
}

$CheckStart = Get-Date
$script:Step = 0
$script:Passed = 0
$script:StepStats = ""

if ($TestsOnly) {
    $script:Total = 1
} elseif ($LintOnly) {
    $script:Total = if ($SkipFunclog) { 5 } else { 6 }
} else {
    $script:Total = if ($SkipFunclog) { 5 } else { 6 }
}

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
        [int]$Code = 1,
        [string]$Fix = ""
    )
    Write-Host ""
    Write-Host "check FAILED: $Name ($($script:Step)/$($script:Total))" -ForegroundColor Red
    if ($Fix) { Write-Host "  fix: $Fix" -ForegroundColor Red }
    exit $Code
}

function Complete-Ok {
    param([string]$Detail = "")
    $elapsed = (Get-Date) - $CheckStart
    Write-Host ""
    Write-Host "check OK  $($script:Passed)/$($script:Total) passed  $(Format-Duration $elapsed)" -ForegroundColor Green
    if ($Detail) { Write-Host "  ($Detail)" }
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

function Get-GoTestStats {
    param([string]$LogPath)
    $content = Get-Content $LogPath -Raw
    $count = ([regex]::Matches($content, '(?m)^(ok|FAIL|\?)')).Count
    if ($count -gt 0) { return "$count packages" }
    return ""
}

function Get-GoTestTargets {
    if ($Group) {
        return (Get-GroupPackages $Group) -join ' '
    }
    return './...'
}

function Step-Gofmt {
    $label = "gofmt"
    Write-StepPrefix
    Write-Host -NoNewline "$label "

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $unformatted = [System.Collections.Generic.List[string]]::new()
    Get-ChildItem -Recurse -Filter '*.go' -File |
        Where-Object { $_.FullName -notmatch '\\vendor\\' } |
        ForEach-Object {
            $line = & gofmt -l $_.FullName
            if ($line) {
                foreach ($path in ($line -split "`n")) {
                    if ($path) { [void]$unformatted.Add($path) }
                }
            }
        }
    $sw.Stop()

    if ($unformatted.Count -gt 0) {
        Write-Host "FAILED" -ForegroundColor Red
        $unformatted
        Fail-Step $label 1 "gofmt -w on the files above"
    }

    $script:Passed++
    Write-OkLine $label $sw.Elapsed
}

function Step-SchedulingBoundary {
    $label = "scheduling boundary"
    Write-StepPrefix
    Write-Host -NoNewline "$label "

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $boundaryHits = & rg -n "gorm|store/|handler/|agents/" pkgs/tasks/scheduling/ -g "*.go" -g "!*_test.go" 2>$null
    $sw.Stop()

    if ($boundaryHits) {
        Write-Host "FAILED" -ForegroundColor Red
        Write-Host "scheduling must not import persistence or transport:" -ForegroundColor Red
        $boundaryHits
        Fail-Step $label 1
    }

    $script:Passed++
    Write-OkLine $label $sw.Elapsed
}

function Step-TestGroupCoverage {
    $label = "test group coverage"
    Write-StepPrefix
    Write-Host -NoNewline "$label "

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        Assert-GroupsCoverAll
    } catch {
        $sw.Stop()
        Write-Host "FAILED" -ForegroundColor Red
        Fail-Step $label 1
    }
    $sw.Stop()

    $script:Passed++
    Write-OkLine $label $sw.Elapsed
}

function Invoke-GoTest {
    $label = if ($Group) { "go test ($Group)" } else { "go test" }
    $targets = Get-GoTestTargets

    Invoke-CapturedStep $label { go test $targets.Split(' ') -count=1 } { param($p) Get-GoTestStats $p }
}

Write-Host "Hamix check (Go)"
Write-Host ""

if ($TestsOnly) {
    Invoke-GoTest
    Complete-Ok
}

Invoke-CapturedStep "check-brand" { & "$PSScriptRoot\check-brand.ps1" }
Step-Gofmt
Invoke-CapturedStep "go vet" { go vet ./... }
Step-SchedulingBoundary

if ($LintOnly) {
    Step-TestGroupCoverage
} else {
    Invoke-GoTest
}

if (-not $SkipFunclog) {
    Invoke-CapturedStep "funclogmeasure" { go run ./cmd/funclogmeasure -enforce }
}

Complete-Ok
