# Brahma Echo + PinchTab Integration

The supplied PinchTab archive is a complete browser-control product, not a Python library. It is a Go HTTP server/CLI with a Chromium runtime. Brahma Echo should treat it as a dedicated browser backend rather than copying its Go packages into the Python application root.

## What is bundled

- Full PinchTab source snapshot under `vendor/pinchtab/`
- Its Go modules, CLI, server, bridge, dashboard, MCP support, security controls, tests, docs, and skills
- `actions/pinchtab_client.py` — Python adapter for Brahma Echo
- `scripts/setup_pinchtab.ps1` — builds `pinchtab.exe`
- `scripts/start_pinchtab.ps1` — starts the local PinchTab server and waits for health

## Major capabilities discovered

- Token-efficient accessibility snapshots with stable element refs such as `e5`
- Navigation, back/forward/reload, tab management
- DOM actions: click, fill, select, press, wait, find
- Mouse actions and drag/drop
- Page text, HTML, attributes, box/visibility/enabled/checked/count inspection
- Screenshots, paired capture, PDF export, downloads/uploads
- Cookies/state/clipboard controls (security-gated)
- Network inspection/export/routing
- Console/error diagnostics
- Emulation: viewport, geolocation, headers, media, offline
- Profiles and isolated browser instances
- Headed or headless Chrome
- Multi-instance orchestration and remote bridge attachment
- Human handoff for CAPTCHA/2FA/login approval
- Optional scheduler for queued multi-step browser actions
- MCP server support
- Site auditing and visual comparison
- Security controls including localhost-first binding, tokens, endpoint gates and IDPI

## Recommended Brahma architecture

```text
Brahma Echo (Python/Qt)
        |
        | actions/pinchtab_client.py
        v
PinchTab HTTP server :9867
        |
        v
PinchTab bridge / Chrome
```

Keep PinchTab's Apache-2.0 license and notices intact inside `vendor/pinchtab`. Do not mix its files into the root Python package or replace Brahma's custom license with the PinchTab license.
