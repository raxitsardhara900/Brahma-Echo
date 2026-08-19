#!/bin/bash
# api-snapshot.sh — Snapshot ref helpers
#
# Provides functions to find and interact with elements from snapshot results.
# All functions operate on $RESULT (set by pt_get /snapshot).

# Find a ref by role and name. Returns empty string if not found.
# Usage: ref=$(find_ref "textbox" "Email")
find_ref() {
  local role="$1"
  local name="$2"
  find_ref_by_role_and_name "$role" "$name" "$RESULT"
}

# Get the value of a node by role and name.
# Usage: val=$(get_value "textbox" "Email")
get_value() {
  local role="$1"
  local name="$2"
  echo "$RESULT" | jq -r "[.nodes[] | select(.role==\"$role\" and .name==\"$name\")][0].value // empty"
}

# Assert a ref exists, set it to a variable, and log result.
# Usage: require_ref "textbox" "Email" EMAIL_REF
# Returns 1 if not found (caller should skip dependent tests).
require_ref() {
  local role="$1"
  local name="$2"
  local varname="$3"
  local ref
  ref=$(find_ref "$role" "$name")

  if [ -z "$ref" ]; then
    fail_assert "could not find $role '$name'"
    printf -v "$varname" '%s' ''
    return 1
  fi

  pass_assert "found $role '$name' → $ref"
  printf -v "$varname" '%s' "$ref"
  return 0
}

# Assert a field has the expected value (substring match).
# Usage: assert_value "textbox" "Email" "test@example.com"
assert_value() {
  local role="$1"
  local name="$2"
  local expected="$3"
  local actual
  actual=$(get_value "$role" "$name")

  if echo "$actual" | grep -qF "$expected"; then
    pass_assert "$name = '$actual'"
  else
    fail_assert "$name: expected '$expected', got '$actual'"
  fi
}
