param(
  [Parameter(Mandatory = $true)]
  [string]$Source
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path $Source)) {
  throw "Source database not found: $Source"
}

$running = Get-Process -Name 'new-api-local' -ErrorAction SilentlyContinue
if ($running) {
  throw "new-api-local is running. Stop it first with: .\stop-local.ps1"
}

$target = Join-Path $PSScriptRoot 'one-api.db'
$backupDir = Join-Path $PSScriptRoot 'db-backups'
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null

if (Test-Path $target) {
  $timestamp = Get-Date -Format 'yyyyMMddHHmmss'
  Copy-Item -Path $target -Destination (Join-Path $backupDir "one-api.$timestamp.db") -Force
}

Copy-Item -Path $Source -Destination $target -Force
Write-Host "SQLite database replaced: $target"
Write-Host 'Run .\check-sqlite.ps1 to verify counts, then .\start-local.ps1 to start.'
