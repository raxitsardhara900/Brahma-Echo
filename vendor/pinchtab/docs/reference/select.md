# Select

Choose an option in a native `<select>` element by selector or ref.

```bash
curl -X POST http://localhost:9867/action \
  -H "Content-Type: application/json" \
  -d '{"kind":"select","ref":"e12","value":"it"}'
# CLI Alternative
pinchtab select e12 it
# Response (use --json for full JSON)
OK
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--snap` | Output snapshot after select |
| `--snap-diff` | Output snapshot diff after select |
| `--text` | Output page text after select |
| `--json` | Full JSON response |
| `--tab` | Target specific tab |

## Option Matching

Matching is forgiving. PinchTab tries these strategies in order:

1. exact `<option value="...">`
2. exact visible text
3. case-insensitive visible text
4. case-insensitive substring of visible text

All of these can work depending on the page:

```bash
pinchtab select e12 uk
pinchtab select e12 "United Kingdom"
pinchtab select e12 "united kingdom"
pinchtab select e12 "Kingdom"
```

Prefer the canonical option value or full visible text when disambiguation matters.

Selector lookup is limited to current frame scope (default: `main`). Use [`/frame`](./frame.md) before iframe selects.

## Related Pages

- [Frame](./frame.md)
- [Snapshot](./snapshot.md)
- [Focus](./focus.md)
