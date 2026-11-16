#Requires -Version 7.4

<#
.SYNOPSIS
    Regenerates the generated code (task gen) and fails if that changed any of it.

.DESCRIPTION
    What CI's generated job checks, run locally by `task gen:check`: sqlc's queries
    (backend/internal/db), the OpenAPI document and the SPA's types (frontend/src/api)
    and the plugin's client (plugins/SenpanCompanion/Api) must be what the generators
    make from the code. CI compares the output with the commit; this compares it with
    the files as they were before it ran, so a change not yet committed, its generated
    files regenerated with it, passes as it will in CI.

.EXAMPLE
    pwsh -NoProfile -File scripts/Test-GeneratedCode.ps1
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

$root = Split-Path -Path $PSScriptRoot -Parent
$folders = 'backend/internal/db', 'frontend/src/api', 'plugins/SenpanCompanion/Api'

# Each generated file's path from the repo root and its content's hash.
function Get-GeneratedState {
    $state = @{}
    foreach ($folder in $folders) {
        foreach ($file in Get-ChildItem -LiteralPath (Join-Path $root $folder) -Recurse -File) {
            $relative = [IO.Path]::GetRelativePath($root, $file.FullName) -replace '\\', '/'
            $state[$relative] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash
        }
    }
    $state
}

$before = Get-GeneratedState
Push-Location -LiteralPath $root
try {
    task gen
    if ($LASTEXITCODE -ne 0) {
        Write-Error 'task gen failed.' -ErrorAction Continue
        exit 1
    }
} finally {
    Pop-Location
}
$after = Get-GeneratedState

$changed = @($before.Keys + $after.Keys | Sort-Object -Unique | Where-Object { $before[$_] -ne $after[$_] })
if ($changed) {
    Write-Host 'The generated code was stale; task gen rewrote:' -ForegroundColor Red
    $changed | ForEach-Object { Write-Host "  $_" }
    exit 1
}
Write-Host 'The generated code is current.'
