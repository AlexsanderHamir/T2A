# Go test groups for CI matrix and scoped local runs.
# Source of truth for which packages belong to core/tasks/agents/harness.

function Get-RepoPackages {
    go list ./cmd/... ./internal/... ./pkgs/...
}

function Get-GroupPackages {
    param([Parameter(Mandatory)][string]$Group)

    switch ($Group) {
        'core' {
            go list ./cmd/... ./internal/... ./pkgs/repo/... ./pkgs/gitcore/... ./pkgs/gitexec/... ./pkgs/gitwork/...
        }
        'tasks' {
            go list ./pkgs/tasks/... | Where-Object { $_ -notmatch '/agentreconcile$' }
        }
        'agents' {
            go list ./pkgs/agents/... ./pkgs/tasks/agentreconcile/... | Where-Object { $_ -notmatch '/harness' }
        }
        'harness' {
            go list ./pkgs/agents/harness/...
        }
        default {
            $valid = (Get-GroupNames) -join ' '
            Write-Error "unknown test group: $Group (valid: $valid)"
            exit 2
        }
    }
}

function Get-GroupNames {
    'core', 'tasks', 'agents', 'harness'
}

function Assert-GroupsCoverAll {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "go not on PATH"
    }

    $all = @(Get-RepoPackages | Sort-Object -Unique)
    if ($all.Count -eq 0) {
        throw "go list returned no packages; is the module tree intact?"
    }
    $grouped = @()
    foreach ($g in Get-GroupNames) {
        $grouped += @(Get-GroupPackages $g)
    }
    $grouped = @($grouped | Sort-Object -Unique)

    $missing = @($all | Where-Object { $_ -notin $grouped })
    $extra = @($grouped | Where-Object { $_ -notin $all })

    if ($missing.Count -gt 0 -or $extra.Count -gt 0) {
        Write-Host "test group coverage mismatch:" -ForegroundColor Red
        if ($missing.Count -gt 0) {
            Write-Host "  not assigned to any group:"
            $missing | ForEach-Object { Write-Host "    $_" }
        }
        if ($extra.Count -gt 0) {
            Write-Host "  assigned but not in repo:"
            $extra | ForEach-Object { Write-Host "    $_" }
        }
        Write-Host "  fix: scripts/test-groups.ps1"
        throw "test group coverage mismatch"
    }
}
