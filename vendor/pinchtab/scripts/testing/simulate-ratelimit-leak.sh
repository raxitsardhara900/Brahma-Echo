#!/usr/bin/env bash
set -euo pipefail

# Flood the server with requests from unique IPs to stress-test
# the rate-limit bucket map. Without the eviction fix (#94),
# this causes unbounded memory growth.
#
# Usage:
#   ./scripts/simulate-ratelimit-leak.sh [host:port] [unique-ips]

HOST="${1:-localhost:9867}"
UNIQUE_IPS="${2:-5000}"
BATCH_SIZE=50

echo "Rate-limit stress test — ${UNIQUE_IPS} unique IPs → ${HOST}"
echo "Monitor: curl http://${HOST}/metrics | jq .metrics.rateBucketHosts"
echo ""

for i in $(seq 1 "$UNIQUE_IPS"); do
  ip="10.$((i / 65536 % 256)).$((i / 256 % 256)).$((i % 256))"
  curl -s -o /dev/null -H "X-Forwarded-For: ${ip}" "http://${HOST}/help" &

  if (( i % BATCH_SIZE == 0 )); then
    wait
    printf "\rSent %d/%d" "$i" "$UNIQUE_IPS"
  fi
done
wait
echo -e "\rDone — sent ${UNIQUE_IPS} requests"
