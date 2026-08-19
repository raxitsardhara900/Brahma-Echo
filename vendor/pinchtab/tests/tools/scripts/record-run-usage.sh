#!/usr/bin/env bash
#
# Records exact run-level usage for a benchmark report.
#
# Usage:
#   ./record-run-usage.sh --type pinchtab|agent-browser \
#     --provider anthropic \
#     --input-tokens 123 \
#     --output-tokens 45 \
#     --cache-creation-input-tokens 67 \
#     --cache-read-input-tokens 890 \
#     [--request-count 12] \
#     [--report-file path]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/../../benchmark"
RESULTS_DIR="${BENCHMARK_DIR}/results"
CURRENT_AGENT_PTR="${RESULTS_DIR}/current_pinchtab_report.txt"
CURRENT_AGENT_BROWSER_PTR="${RESULTS_DIR}/current_agent_browser_report.txt"

REPORT_TYPE=""
REPORT_FILE=""
PROVIDER=""
SOURCE="external"
REQUEST_COUNT=0
INPUT_TOKENS=0
OUTPUT_TOKENS=0
CACHE_CREATION_INPUT_TOKENS=0
CACHE_READ_INPUT_TOKENS=0

resolve_current_report() {
  local ptr="$1"
  if [[ -f "${ptr}" ]]; then
    tr -d '[:space:]' < "${ptr}"
    return 0
  fi
  return 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --type)
      REPORT_TYPE="$2"
      shift 2
      ;;
    --report-file)
      REPORT_FILE="$2"
      shift 2
      ;;
    --provider)
      PROVIDER="$2"
      shift 2
      ;;
    --source)
      SOURCE="$2"
      shift 2
      ;;
    --request-count)
      REQUEST_COUNT="$2"
      shift 2
      ;;
    --input-tokens)
      INPUT_TOKENS="$2"
      shift 2
      ;;
    --output-tokens)
      OUTPUT_TOKENS="$2"
      shift 2
      ;;
    --cache-creation-input-tokens)
      CACHE_CREATION_INPUT_TOKENS="$2"
      shift 2
      ;;
    --cache-read-input-tokens)
      CACHE_READ_INPUT_TOKENS="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

if [[ -z "${PROVIDER}" ]]; then
  echo "ERROR: --provider is required"
  exit 1
fi

if [[ -z "${REPORT_FILE}" ]]; then
  case "${REPORT_TYPE}" in
    pinchtab|agent)
      REPORT_FILE="$(resolve_current_report "${CURRENT_AGENT_PTR}" || true)"
      [[ -n "${REPORT_FILE}" ]] || REPORT_FILE=$(ls -t "${RESULTS_DIR}"/pinchtab_benchmark_*.json 2>/dev/null | head -1)
      ;;
    agent-browser|agent_browser)
      REPORT_FILE="$(resolve_current_report "${CURRENT_AGENT_BROWSER_PTR}" || true)"
      [[ -n "${REPORT_FILE}" ]] || REPORT_FILE=$(ls -t "${RESULTS_DIR}"/agent_browser_benchmark_*.json 2>/dev/null | head -1)
      ;;
    *)
      echo "ERROR: --type must be 'pinchtab' or 'agent-browser' unless --report-file is provided"
      exit 1
      ;;
  esac
fi

if [[ -z "${REPORT_FILE}" || ! -f "${REPORT_FILE}" ]]; then
  echo "ERROR: no benchmark report found"
  exit 1
fi

TOTAL_INPUT_TOKENS=$((INPUT_TOKENS + CACHE_CREATION_INPUT_TOKENS + CACHE_READ_INPUT_TOKENS))
TOTAL_TOKENS=$((TOTAL_INPUT_TOKENS + OUTPUT_TOKENS))

TMP_FILE=$(mktemp)
jq \
  --arg provider "${PROVIDER}" \
  --arg source "${SOURCE}" \
  --argjson request_count "${REQUEST_COUNT}" \
  --argjson input_tokens "${INPUT_TOKENS}" \
  --argjson output_tokens "${OUTPUT_TOKENS}" \
  --argjson cache_creation_input_tokens "${CACHE_CREATION_INPUT_TOKENS}" \
  --argjson cache_read_input_tokens "${CACHE_READ_INPUT_TOKENS}" \
  --argjson total_input_tokens "${TOTAL_INPUT_TOKENS}" \
  --argjson total_tokens "${TOTAL_TOKENS}" \
  '
  .benchmark.runner = $source |
  .run_usage = {
    source: $source,
    provider: $provider,
    request_count: $request_count,
    input_tokens: $input_tokens,
    output_tokens: $output_tokens,
    cache_creation_input_tokens: $cache_creation_input_tokens,
    cache_read_input_tokens: $cache_read_input_tokens,
    total_input_tokens: $total_input_tokens,
    total_tokens: $total_tokens
  }
  ' "${REPORT_FILE}" > "${TMP_FILE}"

mv "${TMP_FILE}" "${REPORT_FILE}"

echo "Recorded run usage in ${REPORT_FILE}"
echo "  Provider: ${PROVIDER}"
echo "  Requests: ${REQUEST_COUNT}"
echo "  Input (uncached): ${INPUT_TOKENS}"
echo "  Cache creation input: ${CACHE_CREATION_INPUT_TOKENS}"
echo "  Cache read input: ${CACHE_READ_INPUT_TOKENS}"
echo "  Total input: ${TOTAL_INPUT_TOKENS}"
echo "  Output: ${OUTPUT_TOKENS}"
echo "  Total: ${TOTAL_TOKENS}"
