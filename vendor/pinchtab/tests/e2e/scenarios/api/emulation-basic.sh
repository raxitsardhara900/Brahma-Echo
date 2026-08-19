#!/bin/bash
# emulation-basic.sh — API happy-path emulation scenarios.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/api.sh"

# Navigate to a fixtures page so there is an active tab for emulation commands.
pt_post /navigate -d "{\"url\":\"${FIXTURES_URL}/form.html\"}"

# ─────────────────────────────────────────────────────────────────
start_test "emulation: set viewport 1024x768"

pt_post /emulation/viewport '{"width":1024,"height":768}'
assert_ok "set viewport"
assert_json_eq "$RESULT" '.status' 'applied' "viewport status applied"
assert_json_eq "$RESULT" '.width' '1024' "viewport width"
assert_json_eq "$RESULT" '.height' '768' "viewport height"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "emulation: set geolocation"

pt_post /emulation/geolocation '{"latitude":51.5074,"longitude":-0.1278}'
assert_ok "set geolocation"
assert_json_eq "$RESULT" '.status' 'applied' "geolocation status applied"
assert_json_contains "$RESULT" '.latitude' '51.5074' "latitude set"
assert_json_contains "$RESULT" '.longitude' '-0.1278' "longitude set"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "emulation: offline mode toggle"

pt_post /emulation/offline '{"offline":true}'
assert_ok "enable offline"
assert_json_eq "$RESULT" '.offline' 'true' "offline flag true"
assert_json_eq "$RESULT" '.status' 'offline' "status is offline"

# MUST disable offline immediately to avoid breaking subsequent tests.
pt_post /emulation/offline '{"offline":false}'
assert_ok "disable offline"
assert_json_eq "$RESULT" '.offline' 'false' "offline flag false"
assert_json_eq "$RESULT" '.status' 'online' "status is online"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "emulation: set custom headers"

pt_post /emulation/headers '{"headers":{"X-Custom-Test":"pinchtab-e2e","Accept-Language":"en-GB"}}'
assert_ok "set custom headers"
assert_json_eq "$RESULT" '.status' 'applied' "headers status applied"
assert_json_exists "$RESULT" '.headers' "response has headers field"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "emulation: clear headers with empty map"

pt_post /emulation/headers '{"headers":{}}'
assert_ok "clear headers"
assert_json_eq "$RESULT" '.status' 'applied' "headers cleared"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "emulation: set media prefers-color-scheme dark"

pt_post /emulation/media '{"feature":"prefers-color-scheme","value":"dark"}'
assert_ok "set media feature"
assert_json_eq "$RESULT" '.status' 'applied' "media status applied"
assert_json_eq "$RESULT" '.feature' 'prefers-color-scheme' "feature echoed"
assert_json_eq "$RESULT" '.value' 'dark' "value echoed"

end_test

# ─────────────────────────────────────────────────────────────────
start_test "emulation: invalid JSON to /emulation/viewport returns 400"

pt_post_raw /emulation/viewport '{broken json!!'
assert_http_status "400" "bad JSON returns 400"

end_test
