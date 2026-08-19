# BrowserBench Runbook

Hand this file to a sub-agent that needs to run the BrowserBench harness end-to-end without prior context.

## What you're doing

Running info-extraction tasks from the [Halluminate BrowserBench](https://github.com/Halluminate/browserbench) dataset against PinchTab. Each task pre-navigates a tab to a starting URL, then drives an LLM agent through `./scripts/pt` browser primitives until it emits `FINAL_ANSWER:` or hits a turn cap. Output is a CSV with `success`/`agent_result`/`ground_truth` per task plus per-task artifacts (HAR, screenshot, console log, full command transcript).

## Before you run

Verify these in order — abort if any fails and report which:

1. `docker info >/dev/null 2>&1` — Docker daemon is up.
2. `[ -n "$ANTHROPIC_API_KEY" ]` (or `OPENAI_API_KEY`) — model API key is in env.
3. `cd /Users/bosh/dev/pinchtab` — repo root is the working directory.

## Run the bench

Start with a small slice to confirm the harness works, then scale up:

```bash
# Smoke (5 tasks, fresh container, live progress)
./dev bench browserbench --tasks 5 --verbose

# Real run (30 tasks, reuse container so it's faster)
./dev bench browserbench --tasks 30 --skip-init

# One specific task (debugging)
./dev bench browserbench --task-id 4 --verbose

# Full dataset (≈25–30 min wall time, expensive in tokens)
./dev bench browserbench --skip-init
```

The wrapper calls `go run ./tests/tools/runner browserbench`. Both forms accept the same flags (`./dev bench browserbench --help`).

Most-used flags:

| Flag | Purpose |
|---|---|
| `--tasks N` | Cap how many tasks run (default: all 295) |
| `--task-id ID` | Run exactly one task |
| `--skip-init` | Reuse the existing container instead of `--force-recreate`-ing |
| `--csv-file PATH` | Override the dataset (point at a local snapshot for reproducibility) |
| `--max-turns N` | Per-task agent loop budget (default 80) |
| `--verbose` / `-v` | Stream per-turn tool calls live |
| `--model NAME` | Override the model (default `claude-haiku-4-5-20251001`) |

## Where the results go

```
tests/tools/browserbench/results/
└── pinchtab_browserbench_<TS>.csv          # one row per task
    pinchtab_browserbench_<TS>/             # same name minus extension
    └── task-<id>/
        ├── commands.ndjson                  # agent's full transcript
        └── artifacts/
            ├── final.png  network.har  console.log
            ├── stealth-status.json
            └── autosolver.json
```

CSV cols of interest: `task_id`, `success`, `agent_result`, `ground_truth`, `execution_time`, `error_message`, plus token counters. `success` = case-insensitive substring match between `agent_result` and `ground_truth`.

The whole `results/` tree is gitignored.

## Constraints — DO NOT do these

These are the recurring failure modes from prior runs. Build them into any agent prompt you write or sub-agent you spawn:

- **`./scripts/pt solve` does not exist.** The autosolver runs server-side only; there is no CLI subcommand. Don't attempt it.
- **`pt instances`, `pt instance ...`, `pt server`, `pt config show`, reading `~/.pinchtab/config.json`** all return `403 session_scope_forbidden` inside the bench session. Don't probe them.
- **Don't retry a click that failed with `element is occluded: top=...`.** Either dismiss the overlay (`pt nav --dismiss-banners`, see SKILL.md) or take a different path.
- **Don't navigate off the starting domain** unless the task naturally leads there.

## Reading the result

After the run finishes:

```bash
# pass rate + which tasks failed
python3 - tests/tools/browserbench/results/pinchtab_browserbench_*.csv <<'PY'
import csv, sys, glob
path = sorted(glob.glob(sys.argv[1]))[-1]
rows = list(csv.DictReader(open(path)))
ok = sum(1 for r in rows if r["success"] == "true")
print(f"{ok}/{len(rows)} passed  ({100*ok/len(rows):.1f}%)  -- {path}")
for r in rows:
    if r["success"] != "true":
        print(f"  fail task={r['task_id']:>3}  err={r['error_message'][:60]!r}  url={r['starting_url']}")
PY
```

For per-task diagnosis, the most informative artifact is `task-<id>/commands.ndjson` — every shell call the agent made, with stdout/stderr and exit codes. Look for repeated `unknown command "solve"` (prompt regression), `element is occluded` (banner problem), or `Just a moment...` (Cloudflare wall).

## Report back

When you finish, surface to the caller:

1. The aggregate pass rate (`X/N passed`).
2. The failing task IDs and the dominant failure mode for each (CAPTCHA, IP block, occlusion, timeout, etc.).
3. The output CSV path so they can diff it against earlier runs.

## Known un-winnable tasks

These will fail under any client-side config — flag them rather than burning turns:

- **IP / geo blocks** (need residential proxy): tasks 2 (dickssportinggoods), 98 (apartments), 112 (homes).
- **Cloudflare Turnstile captchas** (need LLM-fallback or a paid solver API key): tasks 86 (studocu), 99 (bbb), 104 (doordash), 248 (hinative), 274 (cambridge), 276 (cars).

Other failures are agent/prompt issues and worth investigating.

## Reference

- Container config: `tests/tools/config/pinchtab-benchmark.json` — `instanceDefaults.stealthLevel`, `autoSolver.*`. Edits applied on next bench run (or `docker compose ... up -d --force-recreate --no-deps pinchtab` from `tests/tools/`).
- Skill the bench prompt loads: `skills/pinchtab/SKILL.md` (read this if you're spawning your own LLM agents — describes the full `./scripts/pt` surface).
- Runner code: `tests/tools/runner/internal/bench/browserbench.go`.
