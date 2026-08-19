#!/usr/bin/env bash
#
# Starts the benchmark Docker services needed for the agent-browser lane and
# initializes a fresh report file for the next run.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCH_DIR="${SCRIPT_DIR}/.."
RESULTS_DIR="${BENCH_DIR}/results"
CURRENT_REPORT_PTR="${RESULTS_DIR}/current_agent_browser_report.txt"
mkdir -p "${RESULTS_DIR}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/agent_browser_benchmark_${TIMESTAMP}.json"

cd "${BENCH_DIR}"

if [[ -z "${BENCHMARK_SKIP_AGENT_BROWSER_RESTART:-}" ]]; then
  echo "Starting benchmark services for agent-browser..."
  docker compose up -d --build fixtures agent-browser

  echo "Waiting for fixtures to respond from inside the agent-browser container..."
  for _ in $(seq 1 30); do
    if docker compose exec -T agent-browser curl -sf http://fixtures/ >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  if ! docker compose exec -T agent-browser curl -sf http://fixtures/ >/dev/null 2>&1; then
    echo "ERROR: fixtures are not reachable from the agent-browser container"
    exit 1
  fi
else
  echo "Skipping agent-browser restart (BENCHMARK_SKIP_AGENT_BROWSER_RESTART=1) — caller has already configured the container."
  if ! docker compose exec -T agent-browser curl -sf http://fixtures/ >/dev/null 2>&1; then
    echo "ERROR: agent-browser is not healthy and BENCHMARK_SKIP_AGENT_BROWSER_RESTART=1 prevents us from restarting it." >&2
    exit 1
  fi
fi

: > "${RESULTS_DIR}/agent_browser_commands.ndjson"

cat > "${REPORT_FILE}" << EOF
{
  "benchmark": {
    "type": "agent-browser",
    "timestamp": "${TIMESTAMP}",
    "driver": "agent-browser",
    "model": "${BENCHMARK_MODEL:-unknown}",
    "runner": "${BENCHMARK_RUNNER:-manual}"
  },
  "totals": {
    "input_tokens": 0,
    "output_tokens": 0,
    "total_tokens": 0,
    "estimated_cost_usd": 0,
    "tool_calls": 0,
    "steps_passed": 0,
    "steps_failed": 0,
    "steps_skipped": 0,
    "steps_answered": 0,
    "steps_verified_passed": 0,
    "steps_verified_failed": 0,
    "steps_verified_skipped": 0,
    "steps_pending_verification": 0
  },
  "run_usage": {
    "source": "none",
    "provider": "",
    "request_count": 0,
    "input_tokens": 0,
    "output_tokens": 0,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "total_input_tokens": 0,
    "total_tokens": 0
  },
  "steps": []
}
EOF

printf '%s\n' "${REPORT_FILE}" > "${CURRENT_REPORT_PTR}"

# Wipe per-step timing state from any prior run so record-step.sh
# computes durations against THIS run's start, not yesterday's.
rm -f "${RESULTS_DIR}"/run_start_*.ms "${RESULTS_DIR}"/last_step_end_*.ms

echo "Initialized agent-browser benchmark report:"
echo "  ${REPORT_FILE}"
echo ""
echo "Next steps:"
echo "  1. Setup and task groups are already inlined in the runner's system prompt — do not cat setup files, index.md, or group-*.md."
echo "  2. The agent-browser skill is refreshed by runner (DownloadAgentBrowserSkill) at lane start and inlined in the system prompt."
echo "  3. Use ./scripts/ab ... to drive agent-browser inside Docker"
echo "  4. Record and verify each completed benchmark step in a single call with:"
echo "     ./scripts/ab step-end <group> <step> answer \"what you saw\" <pass|fail|skip> \"verification notes\""
echo "  5. Report is finalized automatically at end of run"
