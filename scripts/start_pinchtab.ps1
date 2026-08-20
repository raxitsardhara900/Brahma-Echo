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

# Normalize PinchTab config with Python so Windows PowerShell 5.1 never writes
# a UTF-8 BOM. PinchTab's Go JSON parser expects a clean UTF-8 JSON document.
$Python = $null
try { $Python = (Get-Command python -ErrorAction Stop).Source } catch {}
if (-not $Python) {
    try { $Python = (Get-Command py -ErrorAction Stop).Source } catch {}
}
if (-not $Python) {
    throw 'Python 3.12 is required to normalize the PinchTab configuration.'
}

$NormalizeCode = @'
import json
import sys
from pathlib import Path

config = Path(sys.argv[1])
config.parent.mkdir(parents=True, exist_ok=True)

raw = config.read_bytes() if config.exists() else b''
text = ''
if raw:
    # Handle normal UTF-8, UTF-8 BOM, and the common mojibake form created
    # when a UTF-8 BOM is decoded/re-encoded incorrectly.
    for enc in ('utf-8-sig', 'utf-8', 'cp1252'):
        try:
            candidate = raw.decode(enc)
            break
        except UnicodeDecodeError:
            candidate = ''
    text = candidate.lstrip('\ufeff')
    for prefix in ('Ã¯Â»Â¿', 'ï»¿'):
        if text.startswith(prefix):
            text = text[len(prefix):]
            break

if text.strip():
    try:
        data = json.loads(text)
    except Exception:
        # Keep a recoverable copy and regenerate a minimal config.
        backup = config.with_suffix('.json.corrupt')
        backup.write_bytes(raw)
        data = {}
else:
    data = {}

if not isinstance(data, dict):
    data = {}

server = data.setdefault('server', {})
server.setdefault('port', '9867')
server.setdefault('bind', '127.0.0.1')

instance = data.setdefault('instanceDefaults', {})
instance['mode'] = 'headed'

security = data.setdefault('security', {})
allowed = security.get('allowedDomains')
if not isinstance(allowed, list) or not allowed:
    security['allowedDomains'] = ['127.0.0.1', 'localhost', '::1']

# Write UTF-8 WITHOUT BOM.
config.write_text(json.dumps(data, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')
print(f'PinchTab config normalized: {config}')
'@

& $Python -c $NormalizeCode $Config
if ($LASTEXITCODE -ne 0) {
    throw 'Failed to normalize PinchTab config.'
}

# Restart only the bundled PinchTab server so an old headless/corrupt instance
# cannot remain in memory after the configuration change.
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
