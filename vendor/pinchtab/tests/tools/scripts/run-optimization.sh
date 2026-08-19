#!/bin/bash
#
# PinchTab Benchmark Optimization Loop
# Runs both benchmarks, analyzes differences, proposes improvements
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="${SCRIPT_DIR}/.."
BENCHMARK_DIR="${SCRIPT_DIR}/../../benchmark"
RESULTS_DIR="${BENCHMARK_DIR}/results"
mkdir -p "${RESULTS_DIR}"
LOG_FILE="${RESULTS_DIR}/optimization_log.md"
CURRENT_BASELINE_PTR="${RESULTS_DIR}/current_baseline_report.txt"
CURRENT_AGENT_PTR="${RESULTS_DIR}/current_pinchtab_report.txt"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RUN_NUMBER=$(grep -c "^## Run #" "${LOG_FILE}" 2>/dev/null || echo 0)
RUN_NUMBER=$((RUN_NUMBER + 1))

if [[ -n "${BENCHMARK_SKIP_PINCHTAB_RESTART:-}" ]]; then
    echo "=== PinchTab Benchmark Run #${RUN_NUMBER} ==="
else
    echo "=== PinchTab Optimization Run #${RUN_NUMBER} ==="
fi
echo "Timestamp: ${TIMESTAMP}"

cd "${TOOLS_DIR}"

# By default, ensure pinchtab is running with the production-faithful config
# (pinchtab.json, idpi.wrapContent=true). The cost-comparison benchmark lane
# layers docker-compose.benchmark.yml on top to point PINCHTAB_CONFIG at
# pinchtab-benchmark.json — when invoked via `./dev bench pinchtab`,
# dev-bench.sh has already restarted pinchtab with that overlay AND
# verified the active config, then sets BENCHMARK_SKIP_PINCHTAB_RESTART=1
# so we don't clobber that setup by force-recreating without the overlay.
if [[ -z "${BENCHMARK_SKIP_PINCHTAB_RESTART:-}" ]]; then
    echo "Ensuring pinchtab is running with pinchtab.json (production-faithful, idpi.wrapContent=true)..."
    docker compose -f docker-compose.yml down --remove-orphans 2>/dev/null || true
    docker compose -f docker-compose.yml up -d --build --force-recreate pinchtab
    sleep 15

    # Verify PinchTab is healthy
    if ! curl -sf -H "Authorization: Bearer benchmark-token" http://localhost:9867/health > /dev/null; then
        echo "ERROR: PinchTab not responding, restarting..."
        docker compose down
        docker compose up -d --build
        sleep 15
    fi
else
    echo "Skipping pinchtab restart (BENCHMARK_SKIP_PINCHTAB_RESTART=1) — caller has already configured the container."
    # Still confirm health before initializing reports.
    if ! curl -sf -H "Authorization: Bearer benchmark-token" http://localhost:9867/health > /dev/null; then
        echo "ERROR: pinchtab is not healthy and BENCHMARK_SKIP_PINCHTAB_RESTART=1 prevents us from restarting it." >&2
        echo "       The caller (dev-bench.sh) is responsible for bringing pinchtab up before invoking the runner." >&2
        exit 1
    fi
fi

# Initialize reports
BASELINE_REPORT="${RESULTS_DIR}/baseline_${TIMESTAMP}.json"
AGENT_REPORT="${RESULTS_DIR}/pinchtab_benchmark_${TIMESTAMP}.json"

cat > "${BASELINE_REPORT}" << EOF
{
  "benchmark": {
    "type": "baseline",
    "run_number": ${RUN_NUMBER},
    "timestamp": "${TIMESTAMP}",
    "model": "${BENCHMARK_MODEL:-baseline}",
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

cat > "${AGENT_REPORT}" << EOF
{
  "benchmark": {
    "type": "pinchtab",
    "run_number": ${RUN_NUMBER},
    "timestamp": "${TIMESTAMP}",
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

printf '%s\n' "${BASELINE_REPORT}" > "${CURRENT_BASELINE_PTR}"
printf '%s\n' "${AGENT_REPORT}" > "${CURRENT_AGENT_PTR}"

# Clear previous agent command trace
rm -f "${RESULTS_DIR}/pinchtab_commands.ndjson"

# Wipe per-step timing state from any prior run so record-step.sh
# computes durations against THIS run's start, not yesterday's.
rm -f "${RESULTS_DIR}"/run_start_*.ms "${RESULTS_DIR}"/last_step_end_*.ms

echo "Reports initialized:"
echo "  Baseline: ${BASELINE_REPORT}"
echo "  Agent: ${AGENT_REPORT}"
echo ""
echo "Ready for benchmark execution."
echo "Timestamp for this run: ${TIMESTAMP}"
echo "Run number: ${RUN_NUMBER}"
