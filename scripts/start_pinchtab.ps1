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

# Rebuild config from the existing JSON object using UTF-8 without BOM.
# A BOM/legacy-encoded config causes PinchTab to fail before the server starts.
$cfg = $null
if (Test-Path $Config) {
    try {
        $raw = [System.IO.File]::ReadAllText($Config, [System.Text.Encoding]::UTF8)
        $cfg = $raw | ConvertFrom-Json
    } catch {
        Write-Host "PinchTab config invalid; rebuilding a clean config from defaults." -ForegroundColor Yellow
    }
}

if ($null -eq $cfg) {
    $cfg = [pscustomobject]@{}
}

if ($null -eq $cfg.server) {
    $cfg | Add-Member -MemberType NoteProperty -Name server -Value ([pscustomobject]@{}) -Force
}
$cfg.server.port = '9867'
$cfg.server.bind = '127.0.0.1'

if ($null -eq $cfg.browser) {
    $cfg | Add-Member -MemberType NoteProperty -Name browser -Value ([pscustomobject]@{}) -Force
}
if ($null -eq $cfg.browsers) {
    $cfg | Add-Member -MemberType NoteProperty -Name browsers -Value ([pscustomobject]@{}) -Force
}
$cfg.browsers.default = 'chrome'

if ($null -eq $cfg.instanceDefaults) {
    $cfg | Add-Member -MemberType NoteProperty -Name instanceDefaults -Value ([pscustomobject]@{}) -Force
}
$cfg.instanceDefaults.mode = 'headed'

if ($null -eq $cfg.security) {
    $cfg | Add-Member -MemberType NoteProperty -Name security -Value ([pscustomobject]@{}) -Force
}
$cfg.security.allowedDomains = @('127.0.0.1','localhost','::1','youtube.com','*.youtube.com','google.com','*.google.com','openai.com','*.openai.com','github.com','*.github.com')

$json = $cfg | ConvertTo-Json -Depth 50
[System.IO.File]::WriteAllText($Config, $json, (New-Object System.Text.UTF8Encoding($false)))

# Validate the exact bytes PowerShell wrote before starting PinchTab.
try {
    [void]([System.IO.File]::ReadAllText($Config, [System.Text.Encoding]::UTF8) | ConvertFrom-Json)
    Write-Host "PinchTab config validated: $Config"
} catch {
    throw "PinchTab config validation failed: $($_.Exception.Message)"
}

Get-Process pinchtab -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

# Start and capture output so startup failures are visible to the user.
$StdOut = Join-Path $Root 'pinchtab_startup_stdout.txt'
$StdErr = Join-Path $Root 'pinchtab_startup_stderr.txt'
Remove-Item $StdOut,$StdErr -Force -ErrorAction SilentlyContinue

Start-Process -FilePath $Bin -ArgumentList 'server' -WorkingDirectory (Split-Path -Parent $Bin) -WindowStyle Hidden -RedirectStandardOutput $StdOut -RedirectStandardError $StdErr | Out-Null

$deadline = (Get-Date).AddSeconds(20)
do {
    try {
        $r = Invoke-RestMethod -Uri 'http://127.0.0.1:9867/health' -TimeoutSec 2
        Write-Host 'PinchTab server ready.' -ForegroundColor Green
        Write-Host 'PinchTab browser mode: headed (visible Chrome).' -ForegroundColor Green
        exit 0
    } catch {
        Start-Sleep -Milliseconds 500
    }
} while ((Get-Date) -lt $deadline)

Write-Host "`n=== PINCHTAB STARTUP STDOUT ===" -ForegroundColor Yellow
if (Test-Path $StdOut) { Get-Content $StdOut -Tail 100 }
Write-Host "`n=== PINCHTAB STARTUP STDERR ===" -ForegroundColor Red
if (Test-Path $StdErr) { Get-Content $StdErr -Tail 100 }
throw 'PinchTab server did not become ready on http://127.0.0.1:9867'