#!/bin/bash
# capture-basic.sh — `pinchtab capture` CLI happy paths.

GROUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${GROUP_DIR}/../../helpers/cli.sh"

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab capture (default terse output writes image)"

pt_ok nav "${FIXTURES_URL}/table.html"
OUT_FILE="/tmp/e2e-capture-test.jpg"
rm -f "$OUT_FILE"

pt_ok capture --wait none -o "$OUT_FILE"
if [ -f "$OUT_FILE" ]; then
  pass_assert "capture file created at $OUT_FILE"
  rm -f "$OUT_FILE"
else
  fail_assert "capture file not created"
fi

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab capture --json (full envelope)"

pt_ok nav "${FIXTURES_URL}/form.html"
OUT_FILE="/tmp/e2e-capture-json.jpg"
rm -f "$OUT_FILE"

pt_ok capture --json --wait none -o "$OUT_FILE"
JSON="$PT_OUT"

if echo "$JSON" | jq -e '.epoch.domEpoch' >/dev/null 2>&1; then
  pass_assert "--json emits epoch.domEpoch"
else
  fail_assert "--json output missing epoch.domEpoch"
fi

if echo "$JSON" | jq -e '.snapshot.nodes' >/dev/null 2>&1; then
  pass_assert "--json emits snapshot.nodes"
else
  fail_assert "--json output missing snapshot.nodes"
fi

if [ -f "$OUT_FILE" ]; then
  pass_assert "--json mode still wrote image to disk"
  rm -f "$OUT_FILE"
else
  fail_assert "--json mode did not write image"
fi

end_test

# ─────────────────────────────────────────────────────────────────
start_test "pinchtab capture --scale shrinks the image"

pt_ok nav "${FIXTURES_URL}/table.html"
FULL_FILE="/tmp/e2e-capture-full.jpg"
HALF_FILE="/tmp/e2e-capture-half.jpg"
rm -f "$FULL_FILE" "$HALF_FILE"

pt_ok capture --wait none -o "$FULL_FILE"
pt_ok nav "${FIXTURES_URL}/table.html"
pt_ok capture --wait none --scale 0.25 -o "$HALF_FILE"

FULL_SIZE=$(wc -c < "$FULL_FILE")
HALF_SIZE=$(wc -c < "$HALF_FILE")

if [ "$HALF_SIZE" -lt "$FULL_SIZE" ]; then
  pass_assert "--scale 0.25 ($HALF_SIZE bytes) < default ($FULL_SIZE bytes)"
else
  fail_assert "--scale 0.25 did not shrink ($HALF_SIZE vs $FULL_SIZE)"
fi

rm -f "$FULL_FILE" "$HALF_FILE"

end_test
