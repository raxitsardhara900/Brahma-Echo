# Eval

Run JavaScript in the current tab. This endpoint is disabled unless evaluation is explicitly enabled in config.

Enabling `security.allowEvaluate` is a documented, non-default, security-reducing configuration change. It allows arbitrary JavaScript execution in page context and should only be used on trusted systems with authentication and network exposure reviewed explicitly.

```bash
curl -X POST http://localhost:9867/evaluate \
  -H "Content-Type: application/json" \
  -d '{"expression":"document.title"}'
# CLI Alternative
pinchtab eval "document.title"
# Response (default is result value; use --json for full JSON)
Example Domain
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--await-promise` | Resolve returned Promise before responding |
| `--json` | Full JSON response |
| `--tab` | Target specific tab |

## Examples

```bash
pinchtab eval "document.title"
pinchtab eval "document.querySelectorAll('a').length"
pinchtab eval "fetch('/api/data').then(r => r.json())" --await-promise
pinchtab eval "document.title" --json    # {"result":"Example Domain"}
```

## Notes

- Requires `security.allowEvaluate: true`
- Tab-scoped variant: `POST /tabs/{id}/evaluate`
- `/evaluate` is intentionally **not** frame-scoped
- Current `/frame` state does not affect `pinchtab eval`
- For iframe access, your expression must handle that explicitly

## Related Pages

- [Config](./config.md)
- [Frame](./frame.md)
- [Tabs](./tabs.md)
