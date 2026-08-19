# Type

Type text into an element, sending key events as the text is entered.

```bash
curl -X POST http://localhost:9867/action \
  -H "Content-Type: application/json" \
  -d '{"kind":"type","ref":"e8","text":"Ada Lovelace"}'
# CLI Alternative
pinchtab type e8 "Ada Lovelace"
# Response (use --json for full JSON)
OK
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--humanize` | Use humanized per-character keypress timing (overrides instance config) |
| `--json` | Full JSON response |
| `--tab` | Target specific tab |

## Notes

- Use `fill` when you want to set the value more directly
- Accepts unified selectors: `e8`, `#name`, `xpath://input`, `text:Name`
- Selector lookup is limited to current frame scope (default: `main`)
- Use [`/frame`](./frame.md) before iframe typing
- Missing selectors fail immediately; use `pinchtab wait` for async fields (see [`commands.md`](../commands.md))
- For typing into focused element, use `keyboard type`
- Raw keyboard input is the default. To opt a type action into the slower humanized per-character path, pass `humanize:true` in the action JSON or set `instanceDefaults.humanize:true`.

## Related Pages

- [Frame](./frame.md)
- [Fill](./fill.md) — Set input value directly
- [Keyboard](./keyboard.md) — Low-level keyboard input (type at focused element)
- [Snapshot](./snapshot.md)
