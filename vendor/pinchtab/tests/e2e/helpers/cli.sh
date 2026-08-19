#!/bin/bash
# Common CLI E2E entrypoint.

HELPERS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${HELPERS_DIR}/base.sh"

# Clear any stale tab state file to prevent 404 errors when the CLI tries to
# reuse a tab ID that no longer exists in the server.
rm -f "${XDG_STATE_HOME:-$HOME/.local/state}/pinchtab/current-tab" 2>/dev/null || true
rm -f /tmp/pinchtab-current-tab 2>/dev/null || true

E2E_REF_JSON_VAR="PT_OUT"

pt() {
  local tmpout
  tmpout=$(mktemp)
  local tmperr
  tmperr=$(mktemp)
  local first_arg="${1:-}"
  local -a cmd_prefix=()

  case "$first_arg" in
    ""|config|daemon|security|help|--help|version|--version)
      ;;
    *)
      if [ -n "${E2E_SERVER_TOKEN:-}" ]; then
        cmd_prefix=(env "PINCHTAB_TOKEN=${E2E_SERVER_TOKEN}")
      fi
      ;;
  esac

  echo -e "  ${BLUE}→ pinchtab --server $E2E_SERVER $@${NC}"

  set +e
  "${cmd_prefix[@]}" pinchtab --server "$E2E_SERVER" "$@" > "$tmpout" 2> "$tmperr"
  PT_CODE=$?
  set -e

  PT_OUT=$(cat "$tmpout")
  PT_ERR=$(cat "$tmperr")
  rm -f "$tmpout" "$tmperr"

  if [ -n "$PT_OUT" ]; then
    head -5 <<< "$PT_OUT" || true
  fi
}

# Backward-compatible alias used by some scenario scripts.
pt_cli() {
  pt "$@"
}

assert_cli_ok() {
  local desc="${1:-CLI command succeeds}"
  local code="${PT_CODE:-127}"

  if [ "$code" -eq 0 ]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (exit $code)"
    if [ -n "${PT_ERR:-}" ]; then
      echo -e "  ${RED}stderr: ${PT_ERR}${NC}"
    fi
  fi
}

pt_ok() {
  pt "$@"
  if [ "$PT_CODE" -eq 0 ]; then
    pass_assert "exit 0"
  else
    fail_assert "expected exit 0, got $PT_CODE"
    echo -e "  ${RED}stderr: $PT_ERR${NC}"
  fi
}

pt_fail() {
  pt "$@"
  if [ "$PT_CODE" -ne 0 ]; then
    pass_assert "exit $PT_CODE (expected failure)"
  else
    fail_assert "expected non-zero exit, got 0"
  fi
}

assert_output_contains() {
  local expected="$1"
  local desc="${2:-output contains '$expected'}"

  if echo "$PT_OUT" | grep -q "$expected"; then
    pass_assert "$desc"
  else
    fail_assert "$desc"
    echo -e "  ${RED}  output was: $PT_OUT${NC}"
  fi
}

assert_exit_code() {
  local expected="$1"
  local desc="${2:-exit code is $expected}"
  if [ "$PT_CODE" -eq "$expected" ]; then
    pass_assert "$desc (exit $PT_CODE)"
  else
    fail_assert "$desc (expected $expected, got $PT_CODE)"
  fi
}

assert_exit_code_lte() {
  local max="$1"
  local desc="${2:-exit code <= $max}"
  if [ "$PT_CODE" -le "$max" ]; then
    pass_assert "$desc (exit $PT_CODE)"
  else
    fail_assert "$desc (got $PT_CODE)"
  fi
}

assert_json_field_contains() {
  local path="$1"
  local needle="$2"
  local desc="${3:-$path contains '$needle'}"
  local actual
  actual=$(echo "$PT_OUT" | safe_jq -r "$path" 2>/dev/null)
  if [[ "$actual" == *"$needle"* ]]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (got '$actual')"
  fi
}

assert_file_exists() {
  local path="$1"
  local desc="${2:-file exists: $path}"
  if [ -f "$path" ]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (not found)"
  fi
}

config_version_of() {
  local path="$1"
  safe_jq -r '.configVersion // "none"' "$path"
}

assert_config_version() {
  local path="$1"
  local expected="$2"
  local success_desc="${3:-configVersion is $expected}"
  local actual
  actual=$(config_version_of "$path")

  if [ "$actual" = "$expected" ]; then
    pass_assert "$success_desc"
  else
    fail_assert "expected configVersion $expected, got $actual"
  fi
}

assert_config_version_one_of() {
  local path="$1"
  shift
  local actual
  actual=$(config_version_of "$path")

  while [ "$#" -gt 1 ]; do
    local expected="$1"
    local success_desc="$2"
    shift 2
    if [ "$actual" = "$expected" ]; then
      pass_assert "$success_desc"
      return 0
    fi
  done

  fail_assert "unexpected configVersion: $actual"
  return 1
}

# Assert PT_OUT is a bare tab ID. `pinchtab nav|goto|navigate` emits just
# the tab ID on stdout by default (no JSON envelope); pass --json to get
# the full response. See Navigate() in internal/cli/actions/actions_navigate.go.
assert_tab_id() {
  local desc="${1:-returns tab ID}"
  local trimmed
  trimmed=$(echo "$PT_OUT" | tr -d '[:space:]')
  if [[ "$trimmed" =~ ^[A-Fa-f0-9]{16,64}$ ]]; then
    pass_assert "$desc"
  else
    fail_assert "$desc"
    echo -e "  ${RED}  output was: $PT_OUT${NC}"
  fi
}

assert_output_not_contains() {
  local forbidden="$1"
  local desc="${2:-output does not contain '$forbidden'}"

  if echo "$PT_OUT" | grep -q "$forbidden"; then
    fail_assert "$desc"
    echo -e "  ${RED}  output was: $PT_OUT${NC}"
  else
    pass_assert "$desc"
  fi
}

assert_output_json() {
  local desc="${1:-output is valid JSON}"

  if echo "$PT_OUT" | safe_jq . >/dev/null 2>&1; then
    pass_assert "$desc"
  else
    fail_assert "$desc"
    echo -e "  ${RED}  output was: $PT_OUT${NC}"
  fi
}

assert_json_field() {
  local path="$1"
  local expected="$2"
  local desc="${3:-$path equals '$expected'}"
  local actual
  actual=$(echo "$PT_OUT" | safe_jq -r "$path" 2>/dev/null)

  if [ "$actual" = "$expected" ]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (got '$actual')"
  fi
}

assert_output_jq() {
  local expr="$1"
  local success_desc="$2"
  local fail_desc="${3:-$2}"
  shift 3
  assert_ref_json_jq "$expr" "$success_desc" "$fail_desc" "$@"
}

assert_output_has_tab_event() {
  local tab_id="$1"
  local path="$2"
  local success_desc="$3"
  local fail_desc="$4"
  assert_output_jq \
    '.events[] | select(.tabId == $tab and .path == $path)' \
    "$success_desc" \
    "$fail_desc" \
    --arg tab "$tab_id" \
    --arg path "$path"
}

assert_output_all_events_for_tab() {
  local tab_id="$1"
  local success_desc="$2"
  local fail_desc="$3"
  assert_output_jq \
    'all(.events[]?; .tabId == $tab)' \
    "$success_desc" \
    "$fail_desc" \
    --arg tab "$tab_id"
}
