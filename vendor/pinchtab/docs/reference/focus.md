# Focus

Move focus to an element by selector or ref.

```bash
curl -X POST http://localhost:9867/action \
  -H "Content-Type: application/json" \
  -d '{"kind":"focus","ref":"e8"}'
# CLI Alternative
pinchtab focus e8
# Response (use --json for full JSON)
OK
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--css` | CSS selector instead of ref |
| `--json` | Full JSON response |
| `--tab` | Target specific tab |

This is useful before keyboard-only flows such as `press Enter` or `type`.

The CLI accepts unified selector forms: `e8`, `#input`, `xpath://input`, `text:Email`.

Selector lookup is limited to current frame scope (default: `main`). Use [`/frame`](./frame.md) before iframe focus calls.

## Related Pages

- [Frame](./frame.md)
- [Press](./press.md)
- [Type](./type.md)
