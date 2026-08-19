#!/bin/bash
# run.sh - Container executor for explicit E2E scenario files.

set -uo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

usage() {
  echo "usage: /bin/bash /e2e/run.sh scenario=<file>..." >&2
}

HOST_E2E_HELPER="${E2E_HELPER:-}"
HOST_E2E_SCENARIO_DIR="${E2E_SCENARIO_DIR:-}"
HOST_E2E_REQUIRED_COMMANDS="${E2E_REQUIRED_COMMANDS:-}"
HOST_E2E_READY_TARGETS="${E2E_READY_TARGETS:-}"
HOST_E2E_SUMMARY_TITLE="${E2E_SUMMARY_TITLE:-}"

require_commands() {
  local missing=0
  for cmd in "$@"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "missing required command: $cmd" >&2
      missing=1
    fi
  done
  if [ "$missing" -ne 0 ]; then
    echo "one or more required commands are unavailable in this test environment" >&2
    exit 127
  fi
}

# Parse arguments
EXPLICIT_SCENARIOS=()

for arg in "$@"; do
  case "$arg" in
    scenario=*)
      EXPLICIT_SCENARIOS+=("$(basename "${arg#scenario=}")")
      ;;
    *)
      echo "unknown argument: $arg" >&2
      usage
      exit 1
      ;;
  esac
done

if [ "${#EXPLICIT_SCENARIOS[@]}" -eq 0 ]; then
  echo "no scenarios supplied; the Go host runner must pass scenario=<file> arguments" >&2
  usage
  exit 2
fi

missing_env=()
[ -z "${HOST_E2E_HELPER}" ] && missing_env+=("E2E_HELPER")
[ -z "${HOST_E2E_SCENARIO_DIR}" ] && missing_env+=("E2E_SCENARIO_DIR")
[ -z "${HOST_E2E_REQUIRED_COMMANDS}" ] && missing_env+=("E2E_REQUIRED_COMMANDS")
[ -z "${HOST_E2E_READY_TARGETS}" ] && missing_env+=("E2E_READY_TARGETS")
[ -z "${HOST_E2E_SUMMARY_TITLE}" ] && missing_env+=("E2E_SUMMARY_TITLE")
if [ "${#missing_env[@]}" -gt 0 ]; then
  echo "missing required environment: ${missing_env[*]}" >&2
  exit 2
fi

case "${HOST_E2E_HELPER}" in
  api|cli) ;;
  *)
    echo "unknown E2E_HELPER: ${HOST_E2E_HELPER}" >&2
    exit 2
    ;;
esac

if [[ "${HOST_E2E_SCENARIO_DIR}" = /* ]]; then
  GROUP_DIR="${HOST_E2E_SCENARIO_DIR}"
else
  GROUP_DIR="${ROOT_DIR}/${HOST_E2E_SCENARIO_DIR}"
fi

read -r -a REQUIRED_COMMANDS <<< "${HOST_E2E_REQUIRED_COMMANDS}"
require_commands "${REQUIRED_COMMANDS[@]}"

# shellcheck disable=SC1090
source "${ROOT_DIR}/helpers/${HOST_E2E_HELPER}.sh"

# The host Go runner owns scenario selection. This container executor only
# validates and runs the explicit files it was given.
SCENARIO_GROUPS=()

for scenario in "${EXPLICIT_SCENARIOS[@]}"; do
  if [ -f "${GROUP_DIR}/${scenario}" ]; then
    case " ${SCENARIO_GROUPS[*]} " in
      *" ${scenario} "*) ;;
      *) SCENARIO_GROUPS+=("${scenario}") ;;
    esac
  else
    echo "scenario file not found: ${GROUP_DIR}/${scenario}" >&2
    exit 1
  fi
done

# Check we have scenarios to run
if [ "${#SCENARIO_GROUPS[@]}" -eq 0 ]; then
  echo "no scenario files found in: ${GROUP_DIR}" >&2
  exit 1
fi

export E2E_SUMMARY_TITLE="$HOST_E2E_SUMMARY_TITLE"
SUITE_TITLE="$E2E_SUMMARY_TITLE"

wait_for_ready_targets() {
  local target ref timeout token_ref url token
  for target in ${HOST_E2E_READY_TARGETS}; do
    IFS='|' read -r ref timeout token_ref <<< "${target}"
    if [[ "${ref}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      url="${!ref:-}"
    else
      url="${ref}"
    fi
    if [ -z "${url}" ]; then
      continue
    fi
    token=""
    if [ -n "${token_ref:-}" ]; then
      if [[ "${token_ref}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
        token="${!token_ref:-}"
      else
        token="${token_ref}"
      fi
    fi
    if [ -n "${token}" ]; then
      wait_for_instance_ready "${url}" "${timeout:-60}" "${token}"
    else
      wait_for_instance_ready "${url}" "${timeout:-60}"
    fi
  done
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}${SUITE_TITLE}${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Helper: ${HOST_E2E_HELPER}"
echo "Scenarios: ${HOST_E2E_SCENARIO_DIR}"
echo "E2E_SERVER: ${E2E_SERVER}"
echo "FIXTURES_URL: ${FIXTURES_URL}"
if [ -n "${E2E_TEST_FILTER:-}" ]; then
  echo "TEST: ${E2E_TEST_FILTER}"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Waiting for configured targets..."
wait_for_ready_targets

echo ""
# When E2E_TEST_FILTER is set, source only scenario preamble + matching
# start_test...end_test blocks. Lets a single test run end-to-end with the
# scenario's setup intact, no per-helper guards needed.
TEST_FILTER="${E2E_TEST_FILTER:-}"

source_filtered_scenario() {
  local script_path="$1"
  local pattern="$2"
  local script_dir
  script_dir="$(dirname "${script_path}")"
  # The scenarios dir may be read-only (runner mounts ./:/e2e:ro), so we
  # write the filtered tempfile to /tmp. Scenarios resolve helper paths
  # via `GROUP_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)`, which
  # would point at /tmp from the tempfile — so we strip that line and
  # pre-set GROUP_DIR to the original scenario directory instead.
  # BusyBox mktemp (Alpine) needs the XXXXXX at the end of TEMPLATE — no
  # trailing extension. Use an explicit /tmp path so behaviour is consistent
  # across GNU and BusyBox.
  local tmp
  tmp=$(mktemp /tmp/e2e-scenario.XXXXXX)
  printf 'GROUP_DIR=%q\n' "${script_dir}" > "${tmp}"
  awk -v want="${pattern}" '
    BEGIN { in_test=0; capture=0; matched=0 }
    /^[[:space:]]*GROUP_DIR=.*BASH_SOURCE/ { next }
    /^[[:space:]]*start_test[[:space:]]/ {
      in_test=1
      name=$0
      sub(/^[[:space:]]*start_test[[:space:]]+/, "", name)
      gsub(/^["'\'']|["'\'']$/, "", name)
      if (index(name, want) > 0) { capture=1; matched=1; print } else { capture=0 }
      next
    }
    /^[[:space:]]*end_test[[:space:]]*$/ && in_test {
      if (capture) { print }
      in_test=0
      capture=0
      next
    }
    in_test {
      if (capture) { print }
      next
    }
    { print }
    END { exit matched ? 0 : 2 }
  ' "${script_path}" >> "${tmp}"
  local awk_status=$?
  if [ "${awk_status}" -eq 2 ]; then
    rm -f "${tmp}"
    return 2
  fi
  # shellcheck disable=SC1090
  source "${tmp}"
  rm -f "${tmp}"
  return 0
}

# Run scenarios
for script_name in "${SCENARIO_GROUPS[@]}"; do
  script_path="${GROUP_DIR}/${script_name}"
  if [ ! -f "${script_path}" ]; then
    echo "group entry not found: ${script_path}" >&2
    exit 1
  fi

  echo -e "${YELLOW}Running: ${script_name}${NC}"
  echo ""
  CURRENT_SCENARIO_FILE="${script_name%.sh}"
  if [ -n "${TEST_FILTER}" ]; then
    if ! source_filtered_scenario "${script_path}" "${TEST_FILTER}"; then
      echo -e "${MUTED}  no matching test in ${script_name}${NC}"
    fi
  else
    source "${script_path}"
  fi
  echo ""

done

finish_suite
