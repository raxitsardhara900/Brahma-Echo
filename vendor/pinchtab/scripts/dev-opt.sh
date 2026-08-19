#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TOOLS_DIR="${ROOT_DIR}/tests/tools"
BENCHMARK_DIR="${ROOT_DIR}/tests/benchmark"
RESULTS_DIR="${BENCHMARK_DIR}/results"

resolve_current_report() {
  local ptr="$1"
  if [[ -f "${ptr}" ]]; then
    tr -d '[:space:]' < "${ptr}"
    return 0
  fi
  return 1
}

usage() {
  cat <<'EOF'
Usage:
  ./dev opt baseline

Runs the optimization baseline lane (no API keys required).
EOF
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

mode="$1"
shift

cd "${TOOLS_DIR}"

case "${mode}" in
  baseline)
    ./scripts/run-optimization.sh
    ./scripts/baseline.sh
    BASELINE_REPORT="$(resolve_current_report "${RESULTS_DIR}/current_baseline_report.txt")"
    if [[ -n "${BASELINE_REPORT}" && -f "${BASELINE_REPORT}" ]]; then
      echo ""
      echo "Baseline complete:"
      jq -r '"  steps: \(.totals.steps_answered // .totals.steps_passed // 0)/87  verified: \(.totals.steps_verified_passed // 0)/87  elapsed: \(.totals.elapsed_ms // 0)ms"' "${BASELINE_REPORT}"
      echo "  report: ${BASELINE_REPORT}"
    fi
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    echo "ERROR: unknown optimization mode: ${mode}" >&2
    usage
    exit 1
    ;;
esac
