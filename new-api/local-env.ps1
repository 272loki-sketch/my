$ErrorActionPreference = 'Stop'

$repoRoot = $PSScriptRoot
$env:Path = "D:\tools\go\bin;D:\tools\bun\bun-windows-x64;$env:Path"
$env:GOMODCACHE = 'D:\tools\go-cache\pkg\mod'
$env:GOCACHE = 'D:\tools\go-cache\build'
$env:GOPROXY = 'https://goproxy.cn,direct'
$env:BUN_INSTALL = 'D:\tools\bun'
$env:BUN_INSTALL_CACHE_DIR = 'D:\tools\bun-cache'
