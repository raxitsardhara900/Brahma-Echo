# Benchmark Optimization Loop

Single source of truth for how an agent should improve the benchmark and the
PinchTab lane over time.

## Purpose

The loop exists to improve the real agent outcome against a stable benchmark.

That means the agent should:

- understand the current benchmark contract
- compare the agent lane against the latest green baseline
- decide whether the next best move is:
  - a product/code change
  - a skill/docs change
  - a benchmark/task clarification
  - benchmark expansion into new space

## Inputs

The loop should read these files first:

- `../tools/runner/assets/setup-pinchtab.md`
- `../benchmark/index.md`
- `../tools/scripts/baseline.sh`

Useful supporting files:

- `results/optimization_log.md`
- `results/best_score.txt`
- `test-cases-summary.md`

## Baseline

Baseline is **not** an agent task.

Baseline is a deterministic shell verification that proves the benchmark
environment still behaves as expected.

Source of truth:

- `tests/tools/scripts/baseline.sh`

Run baseline directly:

```bash
# From repo root:
./dev opt baseline
```

### When to run baseline

Run baseline when any of these changed:

- PinchTab server or CLI behavior
- fixtures
- `scripts/baseline.sh`
- benchmark Docker setup
- benchmark task contract in a way that must be proven executable

Do **not** treat baseline as part of the recurring agent loop if nothing in the
benchmark environment changed.

## Core Loop

Each loop should do exactly this:

1. Read the latest green baseline result and the latest agent result.
2. Compare them step-by-step.
3. Identify failures where baseline is green and agent is worse.
4. Infer the smallest high-value change.
5. Make one focused change.
6. Re-run the agent lane.
7. Log the outcome.

The loop should prefer one clear change over broad refactors.

## Decision Rules

### Make a code/product change when:

- baseline proves the behavior is wrong or missing in PinchTab
- the CLI/API is too brittle for a task the benchmark legitimately requires
- the agent failure comes from product ergonomics, not just prompt wording
- a small product fix would help many tasks, not just one narrow case

Typical examples:

- wrong endpoint behavior
- poor error messages
- selector resolution problems
- missing CLI affordance
- state handling bug

### Make a skill/docs change when:

- PinchTab can already do the task correctly
- the agent chose the wrong tool or sequence
- the failure is best explained by missing or unclear guidance
- a short instruction/example would likely prevent repetition

Typical examples:

- when to use `text` vs `snap -i -c`
- when to re-snapshot
- how to handle navigation `409`
- how to rely on the current-tab state file or use `--tab <id>`

### Make a benchmark/task change when:

- the intended behavior is correct, but the benchmark wording is ambiguous
- verification is weak or misleading
- the task requires knowledge not present in the lane setup docs
- the benchmark contract should be clarified without changing product behavior

Typical examples:

- task wording too vague
- verification marker unreachable
- expected result not specific enough

### Expand benchmark space when:

- baseline is green
- the latest agent loop no longer reveals an obvious product or skill gap
- the current benchmark is saturated in that area
- an uncovered scenario would increase benchmark value

Expansion should target meaningful new capability areas, not random complexity.

Good expansion targets:

- under-covered interaction types
- state persistence
- multi-step workflows
- accessibility-sensitive flows
- iframe/shadow-dom edge cases
- dynamic content and waiting

## Expansion Rules

When expanding the benchmark:

1. Add or update fixture files in `../tools/fixtures/`.
2. Add the executable baseline case in `../tools/scripts/baseline.sh`.
3. Add the matching shared task in `../benchmark/group-XX.md` and keep `../benchmark/index.md` current.
4. Update `test-cases-summary.md` if it is used as the case inventory.
5. Run baseline and require it to be green before trusting the new case.

Do not add agent-only benchmark cases. Every new benchmark case must have a
working baseline path first.

## Agent Execution Rules

For the PinchTab agent lane:

- use `../tools/runner/assets/setup-pinchtab.md`
- use `../benchmark/index.md` plus the relevant `../benchmark/group-XX.md` files
- use only `./scripts/pt` for browser work
- record and verify with `./scripts/runner step-end` (type auto-detected)

The agent should not use baseline as an execution guide. Baseline is only the
reference oracle and environment check.

## Prioritization

Use this priority order:

1. Fix a real product/code bug.
2. Fix a recurring skill/docs gap.
3. Clarify an ambiguous benchmark/task.
4. Expand the benchmark into new space.

Do not jump to benchmark expansion while a clear product or skill gap is still
unresolved.

## Regression Rule

Compare against `results/best_score.txt`.

- if the new run is worse, treat it as regression work first
- if the new run is equal or better, update the best score

Never hide regressions by weakening the benchmark.

## Loop Output

Each run should append a compact entry to `results/optimization_log.md` with:

- baseline reference used
- agent result
- gap summary
- root cause
- single change made
- why that change was chosen
- next focus

## Non-Goals

Do not:

- run baseline as an agent task
- make multiple unrelated changes in one loop
- weaken tests to improve the score
- expand coverage before baseline is trustworthy
- refactor broadly unless the benchmark problem actually requires it
