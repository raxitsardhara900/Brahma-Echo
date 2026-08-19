$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
$Bin = Join-Path $Root 'vendor\pinchtab\bin\pinchtab.exe'
if (-not (Test-Path $Bin)) {
    & (Join-Path $PSScriptRoot 'setup_pinchtab.ps1')
}

Start-Process -FilePath $Bin -ArgumentList 'server' -WorkingDirectory (Split-Path -Parent $Bin) -WindowStyle Hidden

$deadline = (Get-Date).AddSeconds(15)
do {
    try {
        $r = Invoke-RestMethod -Uri 'http://127.0.0.1:9867/health' -TimeoutSec 2
        Write-Host 'PinchTab server ready.'
        exit 0
    } catch {
        Start-Sleep -Milliseconds 500
    }
} while ((Get-Date) -lt $deadline)

throw 'PinchTab server did not become ready on http://127.0.0.1:9867'
