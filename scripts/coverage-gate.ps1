# Fail when a CI test group's statement coverage is below its floor.
# Usage: .\scripts\coverage-gate.ps1 -Group <name> [-Profile <path>]
#   -Profile  Reuse an existing cover profile (skips go test).

param(
    [Parameter(Mandatory)][string]$Group,
    [string]$Profile = ""
)

$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repo

. (Join-Path $PSScriptRoot "test-groups.ps1")

$null = Get-GroupPackages $Group

$baselinesPath = Join-Path $PSScriptRoot "coverage-baselines.json"
if (-not (Test-Path $baselinesPath)) {
    Write-Error "missing $baselinesPath"
    exit 1
}

$baselines = Get-Content $baselinesPath -Raw | ConvertFrom-Json
$floor = $baselines.$Group
if ($null -eq $floor) {
    Write-Error "no baseline floor for group: $Group"
    exit 1
}

$ownedProfile = $false
if (-not $Profile) {
    $Profile = [System.IO.Path]::GetTempFileName()
    $ownedProfile = $true

    $targets = (Get-GroupPackages $Group) -join ' '
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    go test $targets.Split(' ') -count=1 -coverprofile="$Profile" *> $null
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) {
        if ($ownedProfile) { Remove-Item $Profile -Force -ErrorAction SilentlyContinue }
        Write-Error "${Group}: go test failed (exit $code)"
        exit $code
    }
}

if (-not (Test-Path $Profile)) {
    Write-Error "cover profile not found: $Profile"
    exit 1
}

try {
    $totalLine = (go tool cover -func="$Profile" | Select-Object -Last 1)
    $pctStr = ($totalLine -split '\s+')[-1] -replace '%', ''
    $pct = [double]$pctStr

    if ($pct -lt [double]$floor) {
        Write-Error "${Group}: ${pct}% < floor ${floor}%"
        exit 1
    }

    Write-Host "${Group}: ${pct}% >= floor ${floor}%"
} finally {
    if ($ownedProfile) {
        Remove-Item $Profile -Force -ErrorAction SilentlyContinue
    }
}
