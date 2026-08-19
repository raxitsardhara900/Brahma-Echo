# Idle CPU vs Activity Log Size

Reproduces [#519](https://github.com/pinchtab/pinchtab/issues/519): daemon CPU scales linearly with activity log size due to full-file JSONL rescan every 1s.

## Setup

```bash
docker run -d --name pinchtab-cpu-test -p 19867:9867 \
  -v pinchtab-cpu-vol:/data pinchtab/pinchtab:latest
```

Wait for instance to be ready:

```bash
TOKEN=$(docker exec pinchtab-cpu-test cat /data/.pinchtab/config.json | \
  grep -o '"token": "[^"]*"' | cut -d'"' -f4)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:19867/health
```

## Generate activity log (10k entries)

```bash
for batch in $(seq 1 100); do
  for i in $(seq 1 100); do
    curl -s -H "Authorization: Bearer $TOKEN" \
      -H "X-Agent-Id: agent-$batch" \
      "http://localhost:19867/health" > /dev/null &
  done
  wait
done
```

Verify log size:

```bash
docker exec pinchtab-cpu-test wc -l /data/.pinchtab/activity/events-client-*.jsonl
# expect ~10000 lines, ~2MB
```

## Measure idle CPU

Wait 15s for traffic to settle, then sample:

```bash
for i in $(seq 1 10); do
  docker stats pinchtab-cpu-test --no-stream --format "{{.CPUPerc}}"
  sleep 5
done
```

## Expected results

| Scenario | Unpatched | Patched |
|----------|-----------|---------|
| 10k-line log, idle | 3.4-5.3% sustained | 0.00-0.11% steady-state |
| Extrapolated 30k+ log | 10-12% (as reported) | <0.5% |

## Testing patched binary

Cross-compile and mount over the container binary:

```bash
GOOS=linux GOARCH=arm64 go build -o /tmp/pinchtab-test ./cmd/pinchtab

docker rm -f pinchtab-cpu-test
docker run -d --name pinchtab-cpu-test -p 19867:9867 \
  -v pinchtab-cpu-vol:/data \
  -v /tmp/pinchtab-test:/usr/local/bin/pinchtab \
  pinchtab/pinchtab:latest
```

Re-run the measurement section above and compare.

## Cleanup

```bash
docker rm -f pinchtab-cpu-test
docker volume rm pinchtab-cpu-vol
```

## Root cause

The `IngestPersistedAgentActivity` ticker ran every 1s and called `Query()` which:
1. Called `pruneExpiredFiles()` (ReadDir + mutex) on every invocation
2. Opened the JSONL file and scanned all lines from the beginning
3. Unmarshaled every JSON line and checked the `Since` timestamp filter

With 10k lines this took ~4% CPU; with 30k+ lines it reached 10-12%.

## Fix summary

1. **TailReader**: tracks file byte offset, reads only newly-appended lines (O(new) vs O(total))
2. **Adaptive backoff**: poll interval backs off from 1s to 10s when no new events
3. **Prune rate-limiting**: `pruneExpiredFiles` runs at most once per hour (or on day boundary), not on every Record/Query
4. **Scheduler on-demand start**: workers and reaper only launch on first task Submit
5. **Worker signal channel**: workers block on a channel instead of polling at 50ms
