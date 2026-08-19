# Testing

## Quick Start with dev

The `dev` developer toolkit is the easiest way to run checks and tests:

```bash
./dev                    # Interactive picker
./dev test               # All tests (unit + E2E)
./dev test unit          # Unit tests only
./dev e2e                # Extended suite (all extended tests)
./dev e2e basic          # Basic suite (api + cli + infra)
./dev e2e extended       # Extended suite
./dev e2e smoke          # Smoke suite (smoke scenarios + Docker smoke)
./dev e2e smoke-docker   # Host Docker smoke only
./dev e2e api            # API basic tests
./dev e2e cli            # CLI basic tests
./dev e2e infra          # Infra basic tests
./dev e2e api-extended   # API extended, multi-instance
./dev e2e cli-extended   # CLI extended tests
./dev e2e infra-extended # Infra extended, multi-instance
./dev e2e infra-extended --filter auth  # Extended infra suite filtered to "auth"
./dev check              # All checks (format, vet, build, lint)
./dev check go           # Go checks only
./dev check security     # Gosec security scan
./dev format dashboard   # Run Prettier on dashboard sources
./dev doctor             # Setup dev environment
```

E2E summaries and markdown reports prefix each test with its scenario filename, for example `[auth-extended] auth: login sets session cookie`, so it is easy to see which filename filter to use.

## Unit Tests

```bash
go test ./...
# or
./dev test unit
```

Unit tests are standard Go tests that validate individual packages and functions without launching a full server.

## E2E Tests

End-to-end tests launch a real pinchtab server with Chrome and run e2e-level tests against it. Tests are organized into three parallel groups:

- **api** — Browser control and page interaction (tabs, actions, files)
- **cli** — CLI command tests
- **infra** — System, network, security, stealth, orchestration

### E2E Boundary

The Go runner is the execution boundary for E2E. `go run ./tests/tools/runner e2e ...` owns suite expansion, scenario discovery, manifest metadata, compose service selection, readiness waits, container arguments, host Docker smoke checks, logs, reports, failure accounting, and GitHub Actions outputs. Scenario files and helpers own the actual assertions and API or CLI interactions.

`tests/e2e/scenarios/manifest.json` is metadata, not a scenario list. It only overrides tier, helper, required compose services, readiness targets, and tags. Filename suffixes provide the default tier: `*-basic.sh` is `basic`, `*-smoke.sh` is `smoke`, and every other scenario is `extended`.

Tier meanings:
- `basic` is the fast PR happy path
- `extended` is deeper coverage and includes matching `basic` scenarios
- `smoke` is separate high-setup coverage and does not include `basic` or `extended`

Add new scenarios under `tests/e2e/scenarios/<group>/`, choose the tier by filename, add manifest metadata only for non-default service/readiness/helper/tags, and verify selection with `go run ./tests/tools/runner e2e --suite <suite> --filter <name> --dry-run`.

`--filter` is a case-sensitive scenario selector over file name, manifest key, group, tier, helper, and tags. It runs before compose planning, so unmatched suites are skipped and only required services start. `--test` is narrower: it runs one matching `start_test` block inside the already-selected scenarios.

CI uses `.github/workflows/reusable-e2e.yml` and `.github/workflows/reusable-smoke.yml`, both calling the Go runner directly. The workflow layer decides when to run; the Go layer decides what to run and how to report it.

### Basic Suites

```bash
./dev e2e basic
./dev e2e api
./dev e2e cli
./dev e2e infra
```

Use these on pull requests and during normal development:

- `basic` runs all three basic suites (same as CI PR workflow)
- `api` runs the API `*-basic.sh` groups on the single-instance stack
- `cli` runs the CLI `*-basic.sh` groups on the single-instance stack
- `infra` runs the Infra `*-basic.sh` groups on the single-instance stack

### Extended Suites

```bash
./dev e2e api-extended
./dev e2e cli-extended
./dev e2e infra-extended
```

Extended suites run both `*-basic.sh` and `*-extended.sh` scenarios plus standalone scripts. `api-extended` and `infra-extended` use the multi-instance stack for orchestration coverage.

### Extended Meta-Suite

```bash
./dev e2e
./dev e2e extended
```

Runs `api-extended`, `cli-extended`, `infra-extended`, and `plugin` in sequence. Extended suites include both `*-basic.sh` and `*-extended.sh` scenarios.

### Smoke Suite

```bash
./dev e2e smoke
./dev e2e smoke-orchestrator
./dev e2e smoke-security
./dev e2e smoke-docker
```

Smoke is its own tier: it runs `*-smoke.sh` scenarios plus host-level Docker smoke checks, and it does not include basic or extended scenarios. Use the filtered smoke suites when you only need one smoke lane.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `CI` | _(unset)_ | Set to `true` for longer health check timeouts (60s vs 30s) |

### Temp Directory Layout

Each E2E test run creates a single temp directory under `/tmp/pinchtab-test-*/`:

```
/tmp/pinchtab-test-123456789/
├── pinchtab          # Compiled test binary
├── state/            # Dashboard state (profiles, instances)
└── profiles/         # Chrome user-data directories
```

Everything is cleaned up automatically when tests finish.

## Test File Structure

E2E tests are organized by group and feature:

```
tests/e2e/scenarios/
├── api/           # Browser control, tabs, actions, files
│   ├── browser-basic.sh
│   ├── browser-extended.sh
│   ├── tabs-basic.sh
│   └── ...
├── cli/           # CLI command tests
│   ├── browser-basic.sh
│   ├── browser-extended.sh
│   └── ...
└── infra/         # System, network, security, stealth
    ├── system-basic.sh
    ├── system-extended.sh
    ├── stealth-basic.sh
    ├── orchestrator-extended.sh
    └── ...
```

- `*-basic.sh` is the PR happy-path layer
- `*-extended.sh` adds extra and edge-case coverage
- `*-smoke.sh` covers slow or high-setup production smoke checks
- Standalone scripts (no suffix) run only in extended mode

Docker Compose files:
- `tests/e2e/docker-compose.yml` — single-instance stack for basic tests
- `tests/e2e/docker-compose-multi.yml` — multi-instance stack for extended tests

## E2E Results

The Go e2e runner captures suite output, prints the final suite summary, and writes each suite's summary and markdown report under `tests/e2e/results/`:

- `summary-api.txt` / `report-api.md`
- `summary-api-extended.txt` / `report-api-extended.md`
- `summary-cli.txt` / `report-cli.md`
- `summary-cli-extended.txt` / `report-cli-extended.md`
- `summary-infra.txt` / `report-infra.md`
- `summary-infra-extended.txt` / `report-infra-extended.md`
- `summary-api-smoke.txt` / `report-api-smoke.md`
- `summary-cli-smoke.txt` / `report-cli-smoke.md`
- `summary-infra-smoke.txt` / `report-infra-smoke.md`
- `summary-plugin-smoke.txt` / `report-plugin-smoke.md`
- `summary-docker-smoke.txt` / `report-docker-smoke.md`

The runner clears the target suite files before each run so stale results do not survive into the next suite. It also saves the captured suite output as `output-*.log`, captures compose service logs on failure, and writes GitHub Actions outputs and step summaries when running in CI.

## Writing New E2E Tests

Add new coverage directly to a grouped entrypoint in `tests/e2e/scenarios/api/`, `tests/e2e/scenarios/cli/`, `tests/e2e/scenarios/infra/`, or `tests/e2e/scenarios/plugin/`. Keep `*-basic.sh` focused on the PR happy path, put deeper coverage in the matching `*-extended.sh`, and put slow/high-setup checks in `*-smoke.sh`. Add manifest metadata only when the scenario needs non-default services, readiness targets, helper, tier, or tags.

### Example: Grouped API Entrypoint

```bash
#!/bin/bash

# tests/e2e/scenarios/api/tabs-basic.sh
GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/api.sh"

start_test "tab-scoped snapshot"
# ...

start_test "tab focus"
# ...
end_test
```

## Coverage

Generate coverage for unit tests:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Note: E2E tests are black-box tests and don't contribute to code coverage metrics directly.
