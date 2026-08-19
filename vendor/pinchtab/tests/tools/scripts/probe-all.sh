#!/usr/bin/env bash
# Probe every starting_url in the BrowserBench dataset, recording the final URL/title/snap
# and any captcha pauses. Designed to be run from tests/tools/.
#
# Output CSV: /tmp/bb_probe_full.csv
# Live log:   /tmp/bb_probe_full.log

set -u

SRC=/tmp/bb.csv
OUT=/tmp/bb_probe_full.csv
LOG=/tmp/bb_probe_full.log
SERVER=http://localhost:9867
TOKEN=benchmark-token
PT="$(dirname "$0")/pt"

# fresh CSV with header
echo "task_id,starting_url,final_url,title,status,note" > "$OUT"
: > "$LOG"

# materialize task list to a temp file so child processes can't consume the loop's stdin
TASKS=$(mktemp)
trap 'rm -f "$TASKS"' EXIT
python3 - "$SRC" "$TASKS" <<'PY'
import csv, sys
with open(sys.argv[1], newline="") as fh, open(sys.argv[2], "w") as out:
    r = csv.DictReader(fh)
    for row in r:
        out.write(f"{row['task_id']}\t{row['starting_url']}\n")
PY

while IFS=$'\t' read -r task_id url <&3; do
  ts=$(date +%H:%M:%S)
  printf '[%s] %s -> %s\n' "$ts" "$task_id" "$url" | tee -a "$LOG"

  # navigate (5s timeout via curl-style; rely on pt's own timeout)
  "$PT" nav "$url" >/dev/null 2>&1 || true
  sleep 4

  # check if tab paused for human handoff
  state=$(curl -sS -H "Authorization: Bearer $TOKEN" "$SERVER/tabs" \
    | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("tabs") or [{}])[0].get("status",""))')

  note=""
  if [ "$state" = "paused_handoff" ]; then
    reason=$(curl -sS -H "Authorization: Bearer $TOKEN" "$SERVER/tabs" \
      | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("tabs") or [{}])[0].get("handoffReason",""))')
    printf '  >> PAUSED reason=%s — solve in dashboard, then click resume\n' "$reason" | tee -a "$LOG"
    note="paused:$reason"
    # poll up to 5 minutes for human resume
    for i in $(seq 1 60); do
      sleep 5
      state=$(curl -sS -H "Authorization: Bearer $TOKEN" "$SERVER/tabs" \
        | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("tabs") or [{}])[0].get("status",""))')
      [ "$state" != "paused_handoff" ] && break
    done
    if [ "$state" = "paused_handoff" ]; then
      printf '  >> still paused after 5 min, force-resume + skip\n' | tee -a "$LOG"
      curl -sS -X POST -H "Authorization: Bearer $TOKEN" "$SERVER/resume" >/dev/null 2>&1 || true
      note="$note;timeout"
    fi
  fi

  final_url=$("$PT" url 2>/dev/null | tr -d '\r' | head -1)
  title=$("$PT" title 2>/dev/null | tr -d '\r' | head -1)

  # classify the page
  status="ok"
  lc_title=$(printf '%s' "$title" | tr '[:upper:]' '[:lower:]')
  case "$lc_title" in
    *"just a moment"*|*"one moment"*) status="cloudflare_interstitial" ;;
    *"access forbidden"*|*"access denied"*|*"blocked"*|*"403"*) status="blocked_403" ;;
    *"page not found"*|*"404"*|*"not found"*) status="not_found" ;;
    *"are you a robot"*|*"verify you are human"*|*"please verify"*|*"checking your browser"*) status="bot_check" ;;
    *) status="ok" ;;
  esac

  # CSV-escape via python to be safe with commas in titles
  python3 -c '
import csv, sys
w = csv.writer(sys.stdout)
w.writerow(sys.argv[1:])
' "$task_id" "$url" "$final_url" "$title" "$status" "$note" >> "$OUT"
done 3< "$TASKS"

echo "DONE" | tee -a "$LOG"
