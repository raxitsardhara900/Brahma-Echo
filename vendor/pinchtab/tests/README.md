# Tests

This directory contains benchmark tasks and supporting infrastructure for
measuring AI agent performance with PinchTab and other browser-control surfaces.

## Directory Structure

```
tests/
├── benchmark/        # Benchmark task definitions and results
│   ├── group-XX.md   # Task groups (one file per group)
│   ├── index.md      # Task index
│   └── results/      # Generated reports
│
├── optimization/     # Optimization task definitions (future)
│
└── tools/            # Benchmark infrastructure
    ├── runner/       # Go benchmark runner
    ├── scripts/      # Shell wrappers (pt, ab, baseline.sh, etc.)
    ├── fixtures/     # Test HTML pages
    ├── docker/       # Smoke test Dockerfiles
    └── config/       # PinchTab configuration variants
```

## Quick Start

```bash
# From repo root:

# Deterministic baseline verification (no API keys)
./dev opt baseline

# Agent-driven benchmarks
ANTHROPIC_API_KEY=... ./dev bench pinchtab
ANTHROPIC_API_KEY=... ./dev bench agent-browser
```

## Lanes

| Lane | Command | Description |
|------|---------|-------------|
| baseline | `./dev opt baseline` | Deterministic shell verification |
| pinchtab | `./dev bench pinchtab` | Agent-driven via PinchTab CLI |
| agent-browser | `./dev bench agent-browser` | Agent-driven via agent-browser CLI |

## Task Groups

Tasks are organized into groups in `benchmark/group-XX.md`:

- **Group 0**: Setup & diagnosis
- **Group 1**: Reading & extracting content
- **Group 2**: Search functionality
- **Group 3**: Form interactions
- **Groups 4+**: Advanced scenarios (SPA, auth, modals, etc.)

## Reports

Generated reports appear in `benchmark/results/`:

- `*_benchmark_YYYYMMDD_HHMMSS.json` - Raw JSON data
- `*_summary.md` - Human-readable summary
- `*_commands.ndjson` - Command trace

See `tools/README.md` for infrastructure details.
