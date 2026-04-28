$ErrorActionPreference = 'Stop'
. "$PSScriptRoot\local-env.ps1"

Set-Location "$PSScriptRoot\web"
$env:NODE_OPTIONS = '--max-old-space-size=4096'
bun run build
Write-Host 'Frontend built: D:\newapi\new-api\web\dist'
