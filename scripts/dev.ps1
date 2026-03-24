# taskapi + Vite from repo root: .\scripts\dev.ps1  (needs .env / DATABASE_URL)
$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

function Stop-ListenerOnPort([int]$Port) {
    Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        ForEach-Object { if ($_.OwningProcess) { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue } }
    Start-Sleep -Milliseconds 300
}

$port = if ($env:DEV_TASKAPI_PORT) { [int]$env:DEV_TASKAPI_PORT } else { 8080 }
$exe = Join-Path $repo $(if ($env:OS -match 'Windows') { 'taskapi-dev.exe' } else { 'taskapi-dev' })

Push-Location $repo
$api = $null
try {
    & go mod download
    Set-Location (Join-Path $repo "web")
    & npm install
    Set-Location $repo
    & go build -o $exe "./cmd/taskapi"

    for ($i = 1; $i -le 2; $i++) {
        Stop-ListenerOnPort $port
        $api = Start-Process -FilePath $exe -ArgumentList "-port", "$port" -WorkingDirectory $repo -PassThru -NoNewWindow
        if ($null -eq $api) { throw "failed to start taskapi" }

        $until = (Get-Date).AddSeconds(90)
        $ok = $false
        while ((Get-Date) -lt $until) {
            if ($api.HasExited) { break }
            try {
                $tcp = New-Object System.Net.Sockets.TcpClient
                $tcp.Connect("127.0.0.1", $port)
                $tcp.Close()
                $ok = $true
                break
            } catch {
                Start-Sleep -Milliseconds 150
            }
        }

        if ($ok -and -not $api.HasExited) { break }

        if ($null -ne $api -and -not $api.HasExited) {
            Stop-Process -Id $api.Id -Force -ErrorAction SilentlyContinue
        }

        if ($api.HasExited) {
            $code = $api.ExitCode
            $api = $null
            if ($i -eq 2) { throw "taskapi exited on :$port (exit $code)" }
            continue
        }

        $api = $null
        if ($i -eq 2) { throw "taskapi did not listen on :$port" }
    }

    Set-Location (Join-Path $repo "web")
    npm run dev
} finally {
    Pop-Location
    if ($null -ne $api -and -not $api.HasExited) {
        Stop-Process -Id $api.Id -Force -ErrorAction SilentlyContinue
    }
}
