#!/bin/bash
# orchestrator-extended.sh — API full orchestration scenarios.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/api.sh"

BRIDGE_URL="${E2E_BRIDGE_URL:-}"
BRIDGE_TOKEN="${E2E_BRIDGE_TOKEN:-}"

if [ -z "$BRIDGE_URL" ]; then
  echo "  E2E_BRIDGE_URL not set, skipping orchestrator full scenarios"
  return 0 2>/dev/null || exit 0
fi

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: attach remote bridge and proxy tab traffic"

pt_post /instances/attach-bridge "{\"name\":\"e2e-remote-bridge\",\"baseUrl\":\"${BRIDGE_URL}\",\"token\":\"${BRIDGE_TOKEN}\"}"
assert_http_status "201" "attach bridge"
assert_json_eq "$RESULT" '.attachType' 'bridge' "instance attachType is bridge"
assert_json_eq "$RESULT" '.attached' 'true' "instance is marked attached"
assert_json_eq "$RESULT" '.url' "${BRIDGE_URL}" "instance stores remote bridge URL"

ATTACHED_INST_ID=$(echo "$RESULT" | jq -r '.id // empty')
if [ -n "$ATTACHED_INST_ID" ]; then
  echo -e "  ${GREEN}✓${NC} attached bridge instance id: ${ATTACHED_INST_ID}"
  ((ASSERTIONS_PASSED++)) || true
else
  echo -e "  ${RED}✗${NC} attach response missing instance id"
  ((ASSERTIONS_FAILED++)) || true
fi

pt_post "/instances/${ATTACHED_INST_ID}/tabs/open" "{\"url\":\"${FIXTURES_URL}/index.html\"}"
assert_ok "open tab on attached bridge"
assert_tab_id "attached bridge returned tabId"
ATTACHED_TAB_ID="${TAB_ID}"

pt_get "/tabs/${ATTACHED_TAB_ID}/text?format=text"
assert_ok "proxy text via attached bridge tab route"
assert_contains "$RESULT" "Welcome to the E2E test fixtures." "tab text came back through orchestrator proxy"

pt_get "/instances/${ATTACHED_INST_ID}/tabs"
assert_ok "list tabs for attached bridge instance"
assert_json_length_gte "$RESULT" '.' '1' "attached bridge has at least one tab"

pt_get /instances/tabs
assert_ok "aggregate tabs includes attached bridge"
if echo "$RESULT" | jq -e --arg inst "$ATTACHED_INST_ID" '.[] | select(.instanceId == $inst)' >/dev/null 2>&1; then
  echo -e "  ${GREEN}✓${NC} aggregate tab list includes attached bridge instance"
  ((ASSERTIONS_PASSED++)) || true
else
  echo -e "  ${RED}✗${NC} aggregate tab list missing attached bridge instance"
  ((ASSERTIONS_FAILED++)) || true
fi

pt_post "/instances/${ATTACHED_INST_ID}/stop" '{}'
assert_ok "stop attached bridge instance"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: health shows dashboard mode"

pt_get /health
assert_ok "health"
assert_json_eq "$RESULT" '.mode' 'dashboard'

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: list instances"

pt_get /instances
assert_ok "list instances"
assert_json_length_gte "$RESULT" '.' '1' "at least 1 instance"
INST_ID=$(echo "$RESULT" | jq -r '.[0].id // empty')
if [ -n "$INST_ID" ] && [ "$INST_ID" != "null" ]; then
  pass_assert "selected instance id: $INST_ID"
else
  fail_assert "list response did not include an instance id"
fi

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: get instance by id"

pt_get "/instances/${INST_ID}"
assert_ok "get instance"
assert_json_eq "$RESULT" '.id' "$INST_ID"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: instance logs"

pt_get "/instances/${INST_ID}/logs"
assert_ok "get logs"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: aggregate tabs"

pt_post /navigate "{\"url\":\"${FIXTURES_URL}/index.html\"}"
assert_ok "navigate"

pt_get /instances/tabs
assert_ok "aggregate tabs"
assert_json_length_gte "$RESULT" '.' '1' "at least 1 tab"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: instance tabs"

wait_for_orchestrator_instance_status "${E2E_SERVER}" "${INST_ID}" "running" 10

pt_get "/instances/${INST_ID}/tabs"
assert_ok "instance tabs"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: ID format (inst_ prefix)"

assert_instance_id_prefix "$INST_ID"

end_test

start_test "orchestrator: proxy with query params"

pt_post /navigate '{"url":"'"${FIXTURES_URL}"'/index.html?foo=bar&baz=qux"}'
assert_ok "navigate with query params"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: stop non-existent instance"

pt_post "/instances/nonexistent_xyz/stop" '{}'
assert_not_ok "rejects bad instance id"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "orchestrator: proxy routing"

pt_post /navigate "{\"url\":\"${FIXTURES_URL}/buttons.html\"}"
assert_ok "navigate via proxy"

pt_get /snapshot
assert_ok "snapshot via proxy"
assert_json_exists "$RESULT" '.nodes' "has nodes"

end_test
