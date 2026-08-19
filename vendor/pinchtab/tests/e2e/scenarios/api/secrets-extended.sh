#!/bin/bash
# secrets-extended.sh — regression coverage for secret leakage in action responses.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/api.sh"

secrets_navigate() {
  pt_post /navigate -d "{\"url\":\"${FIXTURES_URL}/secrets.html\"}" > /dev/null
  assert_ok "navigate"
}

assert_field_value() {
  local selector="$1"
  local expected="$2"
  local desc="$3"
  local body
  body=$(jq -nc --arg sel "$selector" '{expression: "document.querySelector(\"" + $sel + "\")?.value || \"\""}')
  pt_post /evaluate "$body" > /dev/null
  assert_ok "evaluate $selector"
  assert_json_eq "$RESULT" '.result' "$expected" "$desc"
}

assert_action_response_does_not_echo_secret() {
  local secret="$1"
  local desc="$2"
  assert_not_contains "$RESULT" "$secret" "$desc"
}

focus_selector() {
  local selector="$1"
  pt_post /action -d "{\"kind\":\"focus\",\"selector\":\"$selector\"}" > /dev/null
  assert_ok "focus $selector"
}

# ─────────────────────────────────────────────────────────────────
start_test "fill password selector does not echo raw secret"

secrets_navigate
SECRET="pw-fill-secret-557"
pt_post /action -d "{\"kind\":\"fill\",\"selector\":\"#password\",\"text\":\"$SECRET\"}" > /dev/null
assert_ok "fill password field"
assert_action_response_does_not_echo_secret "$SECRET" "fill password response should not echo raw secret"
assert_field_value "#password" "$SECRET" "password value persisted after fill"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "type password selector does not echo raw secret"

secrets_navigate
SECRET="pw-type-secret-557"
pt_post /action -d "{\"kind\":\"type\",\"selector\":\"#password\",\"text\":\"$SECRET\"}" > /dev/null
assert_ok "type password field"
assert_action_response_does_not_echo_secret "$SECRET" "type password response should not echo raw secret"
assert_field_value "#password" "$SECRET" "password value persisted after type"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "humanized type password selector does not echo raw secret"

secrets_navigate
SECRET="pw-human-secret-557"
pt_post /action -d "{\"kind\":\"type\",\"selector\":\"#password\",\"text\":\"$SECRET\",\"humanize\":true,\"fast\":true}" > /dev/null
assert_ok "humanized type password field"
assert_action_response_does_not_echo_secret "$SECRET" "humanized type password response should not echo raw secret"
assert_field_value "#password" "$SECRET" "password value persisted after humanized type"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "keyboard-type password does not echo raw secret"

secrets_navigate
focus_selector "#password"
SECRET="pw-keyboard-secret-557"
pt_post /action -d "{\"kind\":\"keyboard-type\",\"text\":\"$SECRET\"}" > /dev/null
assert_ok "keyboard-type password field"
assert_action_response_does_not_echo_secret "$SECRET" "keyboard-type response should not echo raw secret"
assert_field_value "#password" "$SECRET" "password value persisted after keyboard-type"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "keyboard-inserttext password does not echo raw secret"

secrets_navigate
focus_selector "#password"
SECRET="pw-insert-secret-557"
pt_post /action -d "{\"kind\":\"keyboard-inserttext\",\"text\":\"$SECRET\"}" > /dev/null
assert_ok "keyboard-inserttext password field"
assert_action_response_does_not_echo_secret "$SECRET" "keyboard-inserttext response should not echo raw secret"
assert_field_value "#password" "$SECRET" "password value persisted after keyboard-inserttext"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "fill one-time-code field does not echo raw secret"

secrets_navigate
SECRET="654321"
pt_post /action -d "{\"kind\":\"fill\",\"selector\":\"#otp-code\",\"text\":\"$SECRET\"}" > /dev/null
assert_ok "fill otp field"
assert_action_response_does_not_echo_secret "$SECRET" "fill one-time-code response should not echo raw secret"
assert_field_value "#otp-code" "$SECRET" "otp value persisted after fill"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "fill new-password field does not echo raw secret"

secrets_navigate
SECRET="new-password-secret-557"
pt_post /action -d "{\"kind\":\"fill\",\"selector\":\"#new-password\",\"text\":\"$SECRET\"}" > /dev/null
assert_ok "fill new-password field"
assert_action_response_does_not_echo_secret "$SECRET" "fill new-password response should not echo raw secret"
assert_field_value "#new-password" "$SECRET" "new-password value persisted after fill"

end_test

finish_suite
