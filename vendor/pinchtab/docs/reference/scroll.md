# Scroll

Scroll the current tab or a specific element.

```bash
curl -X POST http://localhost:9867/action \
  -H "Content-Type: application/json" \
  -d '{"kind":"scroll","scrollY":800}'
# Response: {"success":true,"result":{"success":true}}

# CLI Alternative (human-readable by default)
pinchtab scroll down
# Output: OK

pinchtab scroll down --snap        # scroll and output snapshot
pinchtab scroll 800 --snap-diff    # scroll and output snapshot diff
pinchtab scroll 800 --json         # Full JSON response

pinchtab scroll -300               # scroll up 300px
pinchtab scroll --dy -300          # the same, as a flag
pinchtab scroll --dx -120          # scroll left 120px
```

Notes:

- a negative pixel count works either way — `pinchtab scroll -300` and `pinchtab scroll --dy -300`
  are the same scroll, and `--tab` may sit in any position in both
- give either the flags or one positional, never both, and only one positional is accepted —
  so `--tab` stays a flag and must not be placed after `--`, where it would be read as a
  positional
- a delta of zero on both axes is refused: it reaches the server as "no delta given", which
  scrolls down by the default 120px rather than doing nothing. An explicit zero on one axis
  (`--dy 0 --dx 500`) is a real scroll and passes through

- use `--snap` to output an interactive snapshot after scrolling
- use `--snap-diff` to output only the changes from the previous snapshot
- the top-level CLI also accepts a pixel value such as `pinchtab scroll 800`
- the raw API uses `scrollY` and `scrollX` for page scrolling
- the raw API can also target an element with `ref` or `selector`
- selector lookup is limited to the current frame scope; the default scope is `main`
- use [`/frame`](./frame.md) or `pinchtab frame` before selector-based iframe scrolling

## Related Pages

- [Frame](./frame.md)
- [Snapshot](./snapshot.md)
- [Text](./text.md)
