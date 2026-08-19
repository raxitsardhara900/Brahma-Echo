#!/bin/bash
# tabs-extended.sh — CLI advanced tab scenarios.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/cli.sh"

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab back (no history)"

pt_ok back
# Terse mode outputs the current URL (or "OK" if no URL).
if [[ "$PT_OUT" == *"://"* ]] || [[ "$PT_OUT" == *"OK"* ]]; then
  pass_assert "back returns URL or OK"
else
  fail_assert "back returns URL or OK"
  echo -e "  ${RED}  output was: $PT_OUT${NC}"
fi

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab back (navigate two pages then back)"

pt_ok nav "${FIXTURES_URL}/index.html"
TAB_ID=$(echo "$PT_OUT" | tr -d '[:space:]')

pt_ok nav "${FIXTURES_URL}/form.html" --tab "$TAB_ID"

pt_ok back --tab "$TAB_ID"
assert_output_contains "index.html" "back returned to index.html"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab forward"

pt_ok forward --tab "$TAB_ID"
assert_output_contains "form.html" "forward returned to form.html"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab reload"

pt_ok reload --tab "$TAB_ID"
# Reload reports the landed URL, matching back and forward.
assert_output_contains "form.html" "reload returned to form.html"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab tab (list)"

pt_ok nav "${FIXTURES_URL}/form.html"
pt_ok tab --json
assert_output_json

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab tab returns valid JSON array"

pt_ok tab --json
assert_output_json "tabs output is valid JSON"
assert_output_contains "tabs" "response contains tabs field"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab nav --new-tab + close roundtrip"

pt_ok nav "${FIXTURES_URL}/index.html" --new-tab
TAB_ID=$(echo "$PT_OUT" | tr -d '[:space:]')

pt_ok tab close "$TAB_ID"

pt_ok tab
assert_output_not_contains "$TAB_ID" "closed tab no longer in list"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab tab close with no args → error"

pt_fail tab close

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab tab close nonexistent → error"

pt_fail tab close "nonexistent_tab_id_12345"

end_test

# ─────────────────────────────────────────────────────────────────
# Human Handoff CLI Tests
# ─────────────────────────────────────────────────────────────────

start_test "tab handoff: pause and check status"

pt_ok nav --new-tab "${FIXTURES_URL}/buttons.html"
HANDOFF_TAB=$(echo "$PT_OUT" | tr -d '[:space:]')

pt_ok tab handoff "$HANDOFF_TAB" --reason "cli_test"
assert_output_contains "paused" "handoff outputs paused"

pt_ok tab handoff-status "$HANDOFF_TAB"
assert_output_contains "paused_handoff" "status shows paused_handoff"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "tab handoff: resume restores active state"

pt_ok tab resume "$HANDOFF_TAB" --status "completed"
assert_output_contains "resumed" "resume outputs resumed"

pt_ok tab handoff-status "$HANDOFF_TAB"
assert_output_contains "active" "status shows active after resume"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "tab handoff: --json flag returns raw JSON"

pt_ok nav --new-tab "${FIXTURES_URL}/buttons.html"
JSON_TAB=$(echo "$PT_OUT" | tr -d '[:space:]')

pt_ok tab handoff "$JSON_TAB" --reason "json_test" --json
assert_output_json "handoff --json returns valid JSON"
assert_output_contains "paused_handoff" "JSON contains status"

pt_ok tab handoff-status "$JSON_TAB" --json
assert_output_json "handoff-status --json returns valid JSON"

pt_ok tab resume "$JSON_TAB" --json
assert_output_json "resume --json returns valid JSON"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "tab handoff: actions blocked while paused"

pt_ok nav --new-tab "${FIXTURES_URL}/buttons.html"
BLOCK_TAB=$(echo "$PT_OUT" | tr -d '[:space:]')

pt_ok tab handoff "$BLOCK_TAB" --reason "block_test"

pt_fail click "#increment" --tab "$BLOCK_TAB"

pt_ok tab resume "$BLOCK_TAB"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "tab handoff: actions work after resume"

pt_ok wait "#increment" --tab "$BLOCK_TAB"
pt_ok click "#increment" --tab "$BLOCK_TAB"

end_test
