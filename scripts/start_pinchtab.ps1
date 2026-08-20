$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$PinchRoot = Join-Path $Root 'vendor\pinchtab'
$Bin = Join-Path $PinchRoot 'bin\pinchtab.exe'
$ConfigDir = Join-Path $env:APPDATA 'pinchtab'
$Config = Join-Path $ConfigDir 'config.json'

if (-not (Test-Path $Bin)) {
    & (Join-Path $PSScriptRoot 'setup_pinchtab.ps1')
}

New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

# Brahma uses PinchTab for browser automation, so managed Chrome must be visible.
# PinchTab supports instanceDefaults.mode = headed/headless; headed is required
# here because users need to see and interact with the browser window.
if (Test-Path $Config) {
    try {
        $cfg = Get-Content $Config -Raw | ConvertFrom-Json
    } catch {
        $cfg = [pscustomobject]@{}
    }
} else {
    $cfg = [pscustomobject]@{}
}

if ($null -eq $cfg.instanceDefaults) {
    $cfg | Add-Member -MemberType NoteProperty -Name instanceDefaults -Value ([pscustomobject]@{}) -Force
}
$cfg.instanceDefaults.mode = 'headed'

if ($null -eq $cfg.security) {
    $cfg | Add-Member -MemberType NoteProperty -Name security -Value ([pscustomobject]@{}) -Force
}
if ($null -eq $cfg.security.allowedDomains) {
    $cfg.security | Add-Member -MemberType NoteProperty -Name allowedDomains -Value @('127.0.0.1','localhost','::1') -Force
}

$cfg | ConvertTo-Json -Depth 50 | Set-Content -Path $Config -Encoding UTF8

# Restart only the bundled PinchTab server so an old headless instance cannot
# remain in memory after the configuration change.
Get-Process pinchtab -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

Start-Process -FilePath $Bin -ArgumentList 'server' -WorkingDirectory (Split-Path -Parent $Bin) -WindowStyle Hidden | Out-Null

$deadline = (Get-Date).AddSeconds(15)
do {
    try {
        $r = Invoke-RestMethod -Uri 'http://127.0.0.1:9867/health' -TimeoutSec 2
        Write-Host 'PinchTab server ready.'
        Write-Host 'PinchTab browser mode: headed (visible Chrome).'
        exit 0
    } catch {
        Start-Sleep -Milliseconds 500
    }
} while ((Get-Date) -lt $deadline)

throw 'PinchTab server did not become ready on http://127.0.0.1:9867'
