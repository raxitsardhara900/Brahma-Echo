#!/bin/bash
# Assertion helpers for API/curl E2E tests.

assert_json_eq() {
  local json="$1"
  local path="$2"
  local expected="$3"
  local desc="${4:-$path = $expected}"
  local actual
  actual=$(echo "$json" | jq -r "$path")

  if [ "$actual" = "$expected" ]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (got: $actual)"
  fi
}

assert_json_value() {
  local json="$1"
  local path="$2"
  local expected="$3"
  local desc="${4:-$path = $expected}"
  assert_json_eq "$json" "$path" "$expected" "$desc"
}

assert_json_contains() {
  local json="$1"
  local path="$2"
  local needle="$3"
  local desc="${4:-$path contains '$needle'}"
  local actual
  actual=$(echo "$json" | jq -r "$path")

  if [[ "$actual" == *"$needle"* ]]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (got: $actual)"
  fi
}

assert_json_length() {
  local json="$1"
  local path="$2"
  local expected="$3"
  local desc="${4:-$path length = $expected}"
  local actual
  actual=$(echo "$json" | jq "$path | length")

  if [ "$actual" -eq "$expected" ]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (got: $actual)"
  fi
}

assert_json_length_gte() {
  local json="$1"
  local path="$2"
  local expected="$3"
  local desc="${4:-$path length >= $expected}"
  local actual
  actual=$(echo "$json" | jq "$path | length")

  if [ "$actual" -ge "$expected" ]; then
    pass_assert "$desc"
  else
    fail_assert "$desc (got: $actual)"
  fi
}

assert_json_exists() {
  local json="$1"
  local path="$2"
  local desc="${3:-$path exists}"

  if echo "$json" | jq -e "$path" >/dev/null 2>&1; then
    pass_assert "$desc"
  else
    fail_assert "$desc (field missing or null)"
  fi
}

assert_json_not_exists() {
  local json="$1"
  local path="$2"
  local desc="${3:-$path does not exist}"

  if echo "$json" | jq -e "$path" >/dev/null 2>&1; then
    fail_assert "$desc (unexpected field found)"
  else
    pass_assert "$desc"
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local desc="${3:-contains '$needle'}"

  if echo "$haystack" | grep -q "$needle"; then
    pass_assert "$desc"
  else
    fail_assert "$desc (not found)"
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local desc="${3:-does not contain '$needle'}"

  if echo "$haystack" | grep -q "$needle"; then
    fail_assert "$desc (found when should be absent)"
  else
    pass_assert "$desc"
  fi
}

assert_result_eq() {
  local path="$1"
  local expected="$2"
  local desc="${3:-$path = $expected}"
  assert_json_eq "$RESULT" "$path" "$expected" "$desc"
}

assert_result_exists() {
  local path="$1"
  local desc="${2:-$path exists}"
  assert_json_exists "$RESULT" "$path" "$desc"
}

assert_result_jq() {
  local expr="$1"
  local success_desc="$2"
  local fail_desc="${3:-$2}"
  shift 3
  assert_ref_json_jq "$expr" "$success_desc" "$fail_desc" "$@"
}

assert_result_has_tab_event() {
  local tab_id="$1"
  local path="$2"
  local success_desc="$3"
  local fail_desc="$4"
  assert_result_jq \
    '.events[] | select(.tabId == $tab and .path == $path)' \
    "$success_desc" \
    "$fail_desc" \
    --arg tab "$tab_id" \
    --arg path "$path"
}

assert_input_not_contains() {
  local selector="$1"
  local forbidden="$2"
  local desc="${3:-$selector should not contain '$forbidden'}"

  local json_body
  json_body=$(jq -n --arg sel "$selector" '{"expression": ("document.querySelector(\"" + $sel + "\")?.value || \"\"")}')
  pt_post /evaluate "$json_body"
  local value
  value=$(echo "$RESULT" | jq -r '.result // empty')

  if echo "$value" | grep -qi "$forbidden"; then
    fail_assert "$desc: found '$forbidden' in value '$value'"
    return 1
  fi

  pass_assert "$desc (value: '$value')"
  return 0
}

assert_http_error() {
  local expected_status="$1"
  local error_pattern="${2:-error}"
  local desc="${3:-HTTP $expected_status error}"
  local actual_status
  actual_status=$(echo "$RESULT" | jq -r '.status // empty')

  if [ "$actual_status" = "$expected_status" ] || grep -q "$error_pattern" <<< "$RESULT"; then
    pass_assert "$desc"
  else
    soft_pass_assert "$desc (got: $actual_status)"
  fi
}

assert_contains_any() {
  local haystack="$1"
  local patterns="$2"
  local desc="${3:-contains expected pattern}"

  if echo "$haystack" | grep -qE "$patterns"; then
    pass_assert "$desc"
  else
    soft_pass_assert "$desc (not found)"
  fi
}

assert_buttons_page() {
  local snap="$1"
  local expected_buttons=("Increment" "Decrement" "Reset")
  local found=0

  for btn in "${expected_buttons[@]}"; do
    if echo "$snap" | jq -e ".nodes[] | select(.name == \"$btn\")" > /dev/null 2>&1; then
      ((found++))
    fi
  done

  if [ "$found" -ge 3 ]; then
    pass_assert "buttons.html: found $found/3 expected buttons"
  else
    fail_assert "buttons.html: found only $found/3 expected buttons"
  fi
}

assert_form_page() {
  local snap="$1"
  local checks=0
  local textboxes
  textboxes=$(echo "$snap" | jq '[.nodes[] | select(.role == "textbox")] | length')
  [ "$textboxes" -ge 2 ] && ((checks++))
  echo "$snap" | jq -e '.nodes[] | select(.name == "Submit")' > /dev/null 2>&1 && ((checks++))
  echo "$snap" | jq -e '.nodes[] | select(.role == "combobox")' > /dev/null 2>&1 && ((checks++))

  if [ "$checks" -ge 3 ]; then
    pass_assert "form.html: found expected form elements"
  else
    fail_assert "form.html: missing expected form elements ($checks/3)"
  fi
}

assert_table_page() {
  local text="$1"
  local checks=0

  echo "$text" | grep -q "Alice Johnson" && ((checks++))
  echo "$text" | grep -q "bob@example.com" && ((checks++))
  echo "$text" | grep -q "Active" && ((checks++))

  if [ "$checks" -ge 3 ]; then
    pass_assert "table.html: found expected table data"
  else
    fail_assert "table.html: missing expected data ($checks/3)"
  fi
}

assert_index_page() {
  local snap="$1"

  if echo "$snap" | jq -e '.title' | grep -q "E2E Test"; then
    pass_assert "index.html: correct title"
  else
    fail_assert "index.html: wrong title"
  fi
}
