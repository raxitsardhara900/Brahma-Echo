# Hover

Move the pointer over an element by selector or ref.

```bash
curl -X POST http://localhost:9867/action \
  -H "Content-Type: application/json" \
  -d '{"kind":"hover","ref":"e5"}'
# CLI Alternative
pinchtab hover e5
# Response (use --json for full JSON)
OK
```

Use this when menus or tooltips appear only after hover.

## CLI Flags

| Flag | Description |
|------|-------------|
| `--css` | CSS selector instead of ref |
| `--x`, `--y` | Hover at specific coordinates |
| `--humanize` | Use humanized bezier+jitter input path (overrides instance config) |
| `--json` | Full JSON response |
| `--tab` | Target specific tab |

The CLI accepts unified selector forms: `e5`, `#menu`, `xpath://button`, `text:Menu`, `find:account menu`.

Selector lookup is limited to current frame scope (default: `main`). Use [`/frame`](./frame.md) before iframe hover calls.

## Related Pages

- [Click](./click.md)
- [Frame](./frame.md)
- [Snapshot](./snapshot.md)
