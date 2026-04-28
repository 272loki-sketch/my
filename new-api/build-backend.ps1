$ErrorActionPreference = 'Stop'
. "$PSScriptRoot\local-env.ps1"

$running = Get-Process -Name 'new-api-local' -ErrorAction SilentlyContinue
if ($running) {
  throw "new-api-local is running. Stop it first with: .\stop-local.ps1"
}

Set-Location $PSScriptRoot
go build -o "$PSScriptRoot\new-api-local.exe" .
Write-Host 'Backend built: D:\newapi\new-api\new-api-local.exe'
