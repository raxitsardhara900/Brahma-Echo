# Press

Send a keyboard key to the current tab.

```bash
curl -X POST http://localhost:9867/action \
  -H "Content-Type: application/json" \
  -d '{"kind":"press","key":"Enter"}'
# CLI Alternative
pinchtab press Enter
# Response (use --json for full JSON)
OK
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--json` | Full JSON response |
| `--tab` | Target specific tab |

Common keys include `Enter`, `Tab`, `Escape`, `ArrowDown`, `ArrowUp`, `Backspace`, `Delete`.

## Related Pages

- [Click](./click.md)
- [Focus](./focus.md)
- [Keyboard](./keyboard.md)
