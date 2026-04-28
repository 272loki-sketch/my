$ErrorActionPreference = 'Stop'

$processes = Get-Process -Name 'new-api-local' -ErrorAction SilentlyContinue
if (-not $processes) {
  Write-Host 'new-api-local is not running.'
  return
}

$processes | Stop-Process
Write-Host "Stopped new-api-local: PID $($processes.Id -join ', ')"
