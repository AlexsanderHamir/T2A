# Hamix brand guard (PowerShell twin of check-brand.sh).
param()

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $repo

$allowlistFile = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) "check-brand-allowlist.txt"
$prefixes = @()
Get-Content $allowlistFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith("#")) {
        $prefixes += ($line -replace "\\", "/")
    }
}

function Test-Allowlisted([string]$filePath) {
    $normalized = ($filePath -replace "\\", "/").TrimStart("./")
    foreach ($prefix in $prefixes) {
        if ($normalized.StartsWith($prefix)) { return $true }
    }
    return $false
}

function Test-BrandPattern([string]$Label, [string]$Pattern, [string[]]$ExtraArgs) {
    $args = @("-n", $Pattern, ".") + $ExtraArgs
    $hits = & rg @args 2>$null
    if (-not $hits) { return }

    $filtered = @()
    foreach ($line in $hits) {
        $file = ($line -split ":", 2)[0] -replace "\\", "/"
        if (-not (Test-Allowlisted $file)) {
            $filtered += $line
        }
    }
    if ($filtered.Count -gt 0) {
        Write-Error "check-brand FAILED: $Label`n$($filtered -join "`n")"
    }
}

$excludeBrandScripts = @("--glob", "!docs/adr/**", "--glob", "!*.png", "--glob", "!scripts/check-brand*")

Test-BrandPattern "retired product word" '\bT2A\b' $excludeBrandScripts
Test-BrandPattern "retired env prefix" 'T2A_' @("--glob", "!docs/adr/**", "--glob", "!scripts/check-brand*")
Test-BrandPattern "retired Go module path" 'github.com/AlexsanderHamir/T2A' @("--glob", "!scripts/check-brand*")
Test-BrandPattern "retired worker scratch dir" '\bt2a-worker\b' @("--glob", "!docs/adr/**")
Test-BrandPattern "retired Prometheus namespace" 'Namespace: "t2a"' @("--glob", "!scripts/check-brand*")
Test-BrandPattern "retired npm package name" '\bt2a-web\b' @()
Test-BrandPattern "retired localStorage prefix" '\bt2a:' @("--glob", "!docs/adr/**")
Test-BrandPattern "retired localStorage key" 't2a_ui_test_mode' @("--glob", "!scripts/check-brand*")
Test-BrandPattern "retired check temp prefix" 't2a-check' @("--glob", "!scripts/check-brand*")

Write-Host "check-brand OK"
exit 0
