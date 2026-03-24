# Start taskapi and the Vite dev server (web/). Stops the API when you exit npm (Ctrl+C).
# Runs go mod download and npm install in web/ first so dependencies are ready.
# Requires: Go, Node/npm, repo-root .env with DATABASE_URL for Postgres.
# Usage (from repo root):  .\scripts\dev.ps1

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

Push-Location $RepoRoot
try {
    & go mod download
    Set-Location (Join-Path $RepoRoot "web")
    & npm install
    Set-Location $RepoRoot

    $api = Start-Process -FilePath "go" `
        -ArgumentList "run", "./cmd/taskapi" `
        -WorkingDirectory $RepoRoot `
        -PassThru `
        -NoNewWindow

    if ($null -eq $api) {
        throw "failed to start taskapi (Start-Process returned null)"
    }

    Set-Location (Join-Path $RepoRoot "web")
    npm run dev
}
finally {
    Pop-Location
    if ($null -ne $api -and -not $api.HasExited) {
        Stop-Process -Id $api.Id -Force -ErrorAction SilentlyContinue
    }
}
