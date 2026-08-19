# Benchmark

This page summarises how PinchTab compares to `agent-browser` on
real agent-loop token cost. Full methodology, per-run tables, and raw
transcripts live in the [benchmark deep dive](./deep-dive/benchmark.md).

## Headline result

PinchTab is cheaper and uses fewer API round trips than agent-browser on
every scope we measured. Percentages below read as "PinchTab is N%
cheaper than agent-browser on this metric":

| Scope                      | n per lane | Cost cheaper | Fewer requests | Fewer tokens |
|----------------------------|-----------:|-------------:|---------------:|-------------:|
| Basic Haiku 4.5 (10 steps) | 5          | **9.5%**     | 23.0%          | 17.9%        |
| Extended Haiku 4.5 (24 steps) | 3       | **19.6%**    | 31.1%          | 26.2%        |
| Extended Sonnet 4.6 (24 steps) | 2      | **20.3%**    | 29.4%          | 25.3%        |

Absolute per-run costs:

| Scope                      | PinchTab avg | agent-browser avg |
|----------------------------|-------------:|------------------:|
| Basic Haiku 4.5            | $0.1024      | $0.1132           |
| Extended Haiku 4.5         | $0.3516      | $0.4372           |
| Extended Sonnet 4.6        | $0.8932      | $1.1204           |

## What's being measured

The number is the **end-to-end token cost of the whole agent loop** —
system prompt, skill, tool calls, tool outputs, model reasoning, and
retries — summed over one complete benchmark run. Usage is read directly
off Anthropic's `usage` object per response; no self-reporting by the
model.

Both lanes run the same task set inside the same Docker Compose
environment, against the same benchmark fixture server, driven by the
same Go runner. The only thing that changes between lanes is the browser
surface the agent talks to and the matching skill that teaches it the
command shapes.

## Why PinchTab wins on cost

Two structural differences drive the gap:

1. **Fewer API round trips.** agent-browser follows a click-then-snapshot
   pattern: every mutation step costs two API calls. PinchTab batches the
   action and the resulting snapshot into one round trip via
   `--snap`/`--snap-diff`, so the same step costs one API call.
2. **Less repeated cache-read.** Those extra round trips on agent-browser
   don't just cost one turn each — they also re-read the cached system
   prompt and skill on every turn. Over a 24-step run the extra
   cache-read tokens dominate the token gap (though not the cost gap,
   since cache reads are only 10% of uncached input pricing).

## How the gap scales

- **Scope:** the gap widens with step count (9.5% on 10 steps, 19.6% on
  24 steps). Every extra step that involves a post-action snapshot adds
  another round trip on agent-browser.
- **Model:** the gap is essentially identical on Haiku 4.5 and Sonnet 4.6
  at extended scope (19.6% vs 20.3%). Stronger reasoning doesn't collapse
  the click→snapshot pattern — the extra round trips are a property of
  the tool surface, not a planning failure the model corrects.

## Caveats

- The 10-step task suite was designed alongside PinchTab's development
  and contains tasks that are awkward-multi-call on agent-browser. A
  co-designed or much larger task set would reduce task-suite bias.
- Both lanes run a trimmed subset of their full skills (header + the one
  reference file the agent actually reaches for) to keep the comparison
  focused on the tool surface rather than documentation weight. A
  production re-run with full skills on both sides would give a
  different number.
- Variance is ~25–30% of mean at the per-run level; n=5 basic / n=3
  extended Haiku / n=2 extended Sonnet give a usable central tendency
  but wide confidence intervals, especially for the Sonnet pair.
- Agent-browser has one outlier per extended run (lae3); excluding it
  would narrow the gap meaningfully.

## Reproducing

```bash
# From the repo root. The benchmark runs in Docker and the entry points
# live in tests/tools/, not tests/benchmark/.

# Deterministic baseline (no API key required)
./dev opt baseline

# PinchTab and agent-browser lanes (Anthropic API key required)
ANTHROPIC_API_KEY=... ./dev bench pinchtab --groups 0,1
ANTHROPIC_API_KEY=... ./dev bench agent-browser --groups 0,1

# Inspect usage (results land under tests/benchmark/results/)
jq '.run_usage' tests/benchmark/results/pinchtab_benchmark_*.json
jq '.run_usage' tests/benchmark/results/agent_browser_benchmark_*.json
```

The shell entry points and Docker compose files live under
`tests/tools/scripts/` and `tests/tools/docker-compose*.yml`; the agent
loop is the Go binary at `tests/tools/runner/`.

See the [benchmark deep dive](./deep-dive/benchmark.md) for per-run
tables, raw transcripts, token breakdowns, variance discussion, and
the full list of measurement caveats.
