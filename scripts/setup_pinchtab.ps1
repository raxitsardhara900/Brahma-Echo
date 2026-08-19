$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$Pinch = Join-Path $Root 'vendor\pinchtab'
$Bin = Join-Path $Pinch 'bin'
New-Item -ItemType Directory -Force -Path $Bin | Out-Null

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is required to build PinchTab. Install Go, then run this script again.'
}

Write-Host 'Building bundled PinchTab...'
Push-Location $Pinch
try {
    go build -o (Join-Path $Bin 'pinchtab.exe') './cmd/pinchtab'
} finally {
    Pop-Location
}

Write-Host 'PinchTab build complete.'
Write-Host 'Binary:' (Join-Path $Bin 'pinchtab.exe')
