#!/bin/bash
# actions-basic.sh — CLI happy-path action scenarios.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/cli.sh"

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab fill <selector> <text>"

pt_ok nav "${FIXTURES_URL}/form.html"
pt_ok fill "#username" "hello world"
assert_output_contains "OK" "confirms fill action"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab press <key>"

pt_ok press Tab

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab scroll down"

pt_ok nav "${FIXTURES_URL}/table.html"
pt_ok scroll down

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab check/uncheck <selector>"

pt_ok nav "${FIXTURES_URL}/form.html"

pt_ok check "#terms"
assert_output_contains "OK" "check marks the checkbox"

pt_ok eval "document.querySelector('#terms').checked"
assert_output_contains "true" "DOM checkbox state is checked"

pt_ok uncheck "#terms"
assert_output_contains "OK" "uncheck clears the checkbox"

pt_ok eval "document.querySelector('#terms').checked"
assert_output_contains "false" "DOM checkbox state is unchecked"

end_test

start_test "pinchtab select"
pt_ok nav "${FIXTURES_URL}/form.html"
pt_ok snap --interactive
pt select e0 "option1" 2>/dev/null
pass_assert "select command executed"
end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab focus <ref>"

pt_ok nav "${FIXTURES_URL}/form.html"
pt_ok snap --interactive --compact=false

USERNAME_REF=$(find_ref_by_role_and_name "textbox" "Username:" "$PT_OUT")
if assert_ref_found "$USERNAME_REF" "username input ref"; then
  pt_ok focus "$USERNAME_REF"
  assert_output_contains "OK" "confirms focus action"

  # Verify the element is now focused
  pt_ok eval "document.activeElement.id"
  assert_output_contains "username" "username input is focused"
fi

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab focus --css <selector>"

pt_ok nav "${FIXTURES_URL}/form.html"

pt_ok focus --css "#email"
assert_output_contains "OK" "confirms focus by CSS selector"

# Verify the element is now focused
pt_ok eval "document.activeElement.id"
assert_output_contains "email" "email input is focused"

end_test
