$ErrorActionPreference = 'Stop'
. "$PSScriptRoot\local-env.ps1"

Set-Location "$PSScriptRoot\web"
bun install
