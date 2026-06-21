# Rebuild the Hamix dev toolchain image (Go + Node in Docker).
#
# Usage (repo root): .\scripts\docker-build.ps1 [flags]
#
# Flags:
#   -NoCache   Pass --no-cache to docker compose build
#   -Help      Show options

param(
    [switch]$Help,
    [switch]$NoCache
)

if ($Help -or $args -contains '--help' -or $args -contains '-h') {
    Get-Content $PSCommandPath | Select-Object -Skip 1 -First 8 | ForEach-Object { $_ -replace '^# ?', '' }
    exit 0
}

$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Push-Location $repo
try {
    $buildArgs = @("compose", "build", "dev")
    if ($NoCache) {
        $buildArgs += "--no-cache"
    }
    & docker @buildArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    Write-Host "Built. Start with: docker compose up"
} finally {
    Pop-Location
}
