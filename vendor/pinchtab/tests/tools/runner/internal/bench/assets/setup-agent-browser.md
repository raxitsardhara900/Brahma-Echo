# Agent Browser Setup

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

- Fixtures: http://fixtures/ (running in Docker as fixtures hostname)
- Session name: benchmark by default (AGENT_BROWSER_SESSION overrides)
- Browser driver: Docker service agent-browser
- Pages: /, /wiki.html, /wiki-go.html, /articles.html, /search.html,
  /form.html, /dashboard.html, /ecommerce.html, /spa.html, /login.html

## Wrapper

Use only ./scripts/ab ... — do not call agent-browser directly.

The wrapper executes agent-browser inside the benchmark Docker service and preserves the shared browser session.


- Session state is automatic — the wrapper keeps one benchmark session across commands, so refs from a prior snapshot stay valid until the DOM changes.
- record and verify each step as you go