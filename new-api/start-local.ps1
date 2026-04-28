$ErrorActionPreference = 'Stop'
. "$PSScriptRoot\local-env.ps1"

$exe = Join-Path $PSScriptRoot 'new-api-local.exe'
$logDir = Join-Path $PSScriptRoot 'logs'

if (-not (Test-Path $exe)) {
  & "$PSScriptRoot\build-backend.ps1"
}

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

$existing = Get-Process -Name 'new-api-local' -ErrorAction SilentlyContinue
if ($existing) {
  Write-Host "new-api-local is already running: PID $($existing.Id -join ', ')"
  return
}

$process = Start-Process -FilePath $exe -WorkingDirectory $PSScriptRoot -ArgumentList '--log-dir', $logDir -PassThru
Start-Sleep -Seconds 2

if ($process.HasExited) {
  throw 'new-api-local exited immediately. Check logs under D:\newapi\new-api\logs.'
}

Write-Host "new-api-local started: PID $($process.Id)"
Write-Host 'Open http://localhost:3000'
