# Benchmark Tools

Infrastructure for running structured benchmarks that measure AI agent performance
with PinchTab and other browser-control surfaces.

## Quick Start

```bash
# From repo root:

# Deterministic baseline (no API keys required)
./dev opt baseline

# Agent-driven benchmarks
ANTHROPIC_API_KEY=... ./dev bench pinchtab
ANTHROPIC_API_KEY=... ./dev bench agent-browser

# With options
./dev bench pinchtab --dry-run
ANTHROPIC_API_KEY=... ./dev bench pinchtab --profile common10
ANTHROPIC_API_KEY=... ./dev bench pinchtab --max-input-tokens 50000
ANTHROPIC_API_KEY=... ./dev bench agent-browser --groups 0,1,2,3

# Direct Go runner
ANTHROPIC_API_KEY=... go run ./tests/tools/runner --lane pinchtab --finalize
```

## Directory Structure

```
tests/tools/
├── runner/           # Go benchmark runner (API loop, step recording, verification)
├── scripts/          # Shell wrappers and utilities
│   ├── runner        # Wrapper for Go runner
│   ├── pt            # PinchTab Docker wrapper
│   ├── ab            # agent-browser Docker wrapper
│   ├── baseline.sh   # Deterministic baseline verification
│   └── ...
├── fixtures/         # Test HTML pages served at http://fixtures/
├── docker/           # Smoke test Dockerfiles
├── config/           # PinchTab configuration variants
├── docker-compose.yml
└── docker-compose.benchmark.yml
```

## Docker Environment

**The benchmark MUST run against Docker.** Do not use a local pinchtab server.

- **Port**: 9867
- **Token**: `benchmark-token`
- **Fixtures**: http://fixtures/

The Docker setup ensures:
- Reproducible environment every run
- Clean state (no leftover profiles/sessions)
- Latest build from current source
- Isolation from local config

## Lanes

### Baseline
Deterministic shell-script verification. No API keys required.
- Source: `scripts/baseline.sh`
- Use: Confirm benchmark environment works after infrastructure changes

### PinchTab (agent)
Agent-driven lane using the PinchTab CLI via `./scripts/pt`.
- Wrapper: `./scripts/pt`
- Step recording: `./scripts/runner step-end <group> <step> ...`

### agent-browser
Agent-driven lane using the agent-browser CLI via `./scripts/ab`.
- Wrapper: `./scripts/ab`
- Step recording: `./scripts/runner step-end <group> <step> ...`

## Runner Subcommands

```bash
# Full benchmark loop
./scripts/runner --lane pinchtab [options]
./scripts/runner --lane agent-browser [options]

# Step recording (type and report auto-detected from current run)
./scripts/runner step-end <group> <step> answer "..." pass|fail|skip "notes"
./scripts/runner record-step <group> <step> <status> [answer] [notes]
./scripts/runner verify-step <group> <step> <status> [notes]
```

## Reports

Reports are generated in `../benchmark/results/`:

- `pinchtab_benchmark_YYYYMMDD_HHMMSS.json` - Raw JSON data
- `pinchtab_benchmark_YYYYMMDD_HHMMSS_summary.md` - Human-readable summary
- `pinchtab_commands.ndjson` - Command trace for debugging

## Configuration

| File | Purpose |
|------|---------|
| `config/pinchtab.json` | Production-faithful config (optimization baseline) |
| `config/pinchtab-benchmark.json` | Variant with `idpi.wrapContent=false` (cost-comparison lanes) |

The benchmark lane uses `pinchtab-benchmark.json` so the `<untrusted_web_content>`
envelope doesn't inflate per-snapshot tokens.
