# Optimization Log

## 2026-04-20 Entry 1

**Baseline:** baseline_20260420_170428.json (85/85 passed)
**Agent:** pinchtab_benchmark_20260420_185144.json (24/24 passed)

**Gap:** None — agent at 100% on current 6-group benchmark scope.

**Changes made this session:**
1. Fixed `resolveActiveReport()` priority bug — was selecting agent-browser report instead of pinchtab
2. Added "don't run extra commands after step-end" prompt instruction — agent was hallucinating `./scripts/runner report`
3. Added `--snap` and `--snap-diff` flags to scroll command — agent was chaining `scroll && snap`
4. Added better eval error hints for null reference errors
5. Fixed task 2.2 — dropdown task mentioned "File" and "Save" but fixture has "Choose option" and "Alpha/Beta/Gamma"
6. Updated docs: scroll.md, commands.md, benchmark.md, optimization-loop.md

**Why:** These were blocking issues discovered during benchmark runs:
- Report resolution bug caused steps to write to wrong report file
- Agent wasting turns on hallucinated commands
- Agent chaining scroll+snap instead of single command
- Confusing eval errors
- Task/fixture mismatch causing agent confusion

**Result:** 10% token reduction (881k → 792k), 5.5% fewer requests (91 → 86)

**Next focus:** Benchmark expansion — current 6-group scope is saturated (100% pass). Consider adding groups from tests/optimization/ that exercise uncovered scenarios.
