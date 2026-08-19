# PinchTab Setup

## Recording

Record and verify each completed step in a single call:

```bash
./scripts/runner step-end <group> <step> answer "<what you saw>" <pass|fail|skip> "verification notes"
```

For fail or skip records, the 4th arg is the failure/skip note and the verify args are ignored — pass them as skip "":

```bash
./scripts/runner step-end <group> <step> fail "<what went wrong>" skip ""
```

Do not self-grade inside the answer payload. Keep the answer factual.

## Environment

- PinchTab: http://localhost:9867, token: benchmark-token
- Fixtures: http://fixtures/ (running in Docker as fixtures hostname)
- Pages: /, /wiki.html, /wiki-go.html, /articles.html, /search.html,
  /form.html, /dashboard.html, /ecommerce.html, /spa.html, /login.html

## Wrapper

Use only ./scripts/pt ... — do not call pinchtab directly.

The wrapper executes pinchtab inside the benchmark Docker service and forwards PINCHTAB_TOKEN and PINCHTAB_SERVER.

- Tab state is automatic — nav persists the tab ID to a state file, and subsequent commands read it.
- record and verify each step as you go