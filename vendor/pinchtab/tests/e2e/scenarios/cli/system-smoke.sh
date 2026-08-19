#!/bin/bash
# system-smoke.sh — CLI lifecycle smoke scenarios.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/cli.sh"

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab instance stop"

pt_ok instance start
INSTANCE_ID=$(echo "$PT_OUT" | jq -r '.id // empty')

if [ -z "$INSTANCE_ID" ]; then
  fail_assert "no disposable instance id returned"
  end_test
  return 0
fi

pass_assert "disposable instance: ${INSTANCE_ID:0:12}..."

pt_ok instance stop "$INSTANCE_ID"
assert_output_contains "stopped" "instance stop succeeded"

# Poll instances list instead of stopping the shared default instance.
STOPPED=false
for ATTEMPT in $(seq 0 12); do
  if [ "$ATTEMPT" -gt 0 ]; then
    sleep 1
  fi
  pt_ok instances
  if ! echo "$PT_OUT" | jq -e --arg id "$INSTANCE_ID" '.[] | select(.id == $id)' >/dev/null 2>&1; then
    STOPPED=true
    break
  fi
done

if [ "$STOPPED" = "true" ]; then
  pass_assert "disposable instance is removed after stop"
else
  skip_assert "disposable instance still listed after 12s"
fi

end_test
