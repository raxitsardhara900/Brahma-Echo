package e2e

import (
	"strings"
	"testing"
)

func TestParseComposePSReadsBothComposeFormats(t *testing.T) {
	ndjson := []byte(`{"ID":"abc","Service":"pinchtab","State":"exited","ExitCode":137}
{"ID":"def","Service":"fixtures","State":"running","ExitCode":0}`)
	states := parseComposePS(ndjson)
	if len(states) != 2 {
		t.Fatalf("ndjson states = %d, want 2: %+v", len(states), states)
	}
	if states[0].Service != "pinchtab" || states[0].ExitCode != 137 || states[0].ID != "abc" {
		t.Fatalf("first ndjson state = %+v", states[0])
	}

	array := []byte(`[{"ID":"abc","Service":"pinchtab","State":"exited","ExitCode":1}]`)
	states = parseComposePS(array)
	if len(states) != 1 || states[0].ExitCode != 1 {
		t.Fatalf("array states = %+v", states)
	}

	if got := parseComposePS([]byte("   ")); got != nil {
		t.Fatalf("blank output states = %+v, want none", got)
	}
	if got := parseComposePS([]byte("not json")); got != nil {
		t.Fatalf("garbage output states = %+v, want none", got)
	}
}

// The failure this exists for reached the reader as `Post /navigate: EOF` plus a
// `connection refused` on the next call, with nothing naming the dead container.
// A service that is gone must be reported as gone, and an out-of-memory kill
// must be named as one rather than left to be inferred.
func TestServiceDeathReportNamesTheDeadContainerAndOOM(t *testing.T) {
	oom := serviceState{Service: "pinchtab", State: "exited", ExitCode: 137}
	oom.oomKilled = true
	lines := serviceDeathReport([]serviceState{
		{Service: "fixtures", State: "running"},
		oom,
	})
	if len(lines) != 1 {
		t.Fatalf("report lines = %v, want only the dead service", lines)
	}
	if !strings.Contains(lines[0], "pinchtab") {
		t.Fatalf("line does not name the service: %q", lines[0])
	}
	if !strings.Contains(lines[0], "OOM-KILLED") {
		t.Fatalf("an out-of-memory kill must be named: %q", lines[0])
	}

	if got := describeServiceDeath(serviceState{Service: "pinchtab", State: "running"}); got != "" {
		t.Fatalf("a running service must produce no line, got %q", got)
	}

	sigkill := describeServiceDeath(serviceState{Service: "pinchtab", State: "exited", ExitCode: 137})
	if strings.Contains(sigkill, "OOM-KILLED") {
		t.Fatalf("exit 137 alone must not claim an OOM kill: %q", sigkill)
	}
	if !strings.Contains(sigkill, "SIGKILL") {
		t.Fatalf("exit 137 must name the signal: %q", sigkill)
	}

	ownExit := describeServiceDeath(serviceState{Service: "pinchtab", State: "exited", ExitCode: 2})
	if !strings.Contains(ownExit, "exit 2") || strings.Contains(ownExit, "SIGKILL") {
		t.Fatalf("a self-inflicted exit must report its own code: %q", ownExit)
	}
}

// A full Docker VM reaches the reader as an ordinary build failure. It must be
// named, and it must NOT be mistaken for the stale-cache condition, whose
// response is a --no-cache rebuild — the worst possible move on a full disk.
func TestOutOfDiskIsRecognisedAndNotTreatedAsStaleCache(t *testing.T) {
	diskLog := `#45 7.627 compile: writing output: write $WORK/b194/_pkg_.a: no space left on device
#45 ERROR: process "/bin/sh -c go build" did not complete successfully: exit code: 1`
	if !isOutOfDiskLog(diskLog) {
		t.Fatal("a build that ran out of disk was not recognised")
	}
	if isBuildKitCacheFailureLog(diskLog) {
		t.Fatal("an out-of-disk build must not be retried as a stale-cache build")
	}

	cacheLog := "failed to stat active key during commit"
	if isOutOfDiskLog(cacheLog) {
		t.Fatal("a stale-cache failure must not be reported as out of disk")
	}
	if !isBuildKitCacheFailureLog(cacheLog) {
		t.Fatal("the stale-cache condition stopped being recognised")
	}

	if isOutOfDiskLog("") {
		t.Fatal("an empty build log must not claim a full disk")
	}
	if !strings.Contains(outOfDiskRemedy, "docker builder prune") {
		t.Fatalf("the remedy must name a command that reclaims space: %q", outOfDiskRemedy)
	}
}

func TestDockerBinaryFollowsTheResolvedComposeCommand(t *testing.T) {
	if got := dockerBinary([]string{"docker", "compose"}); got != "docker" {
		t.Fatalf("docker binary = %q, want docker", got)
	}
	if got := dockerBinary([]string{"/usr/local/bin/docker", "compose"}); got != "/usr/local/bin/docker" {
		t.Fatalf("docker binary = %q, want the resolved path", got)
	}
	if got := dockerBinary([]string{"docker-compose"}); got != "docker" {
		t.Fatalf("docker binary = %q, want the docker fallback for compose v1", got)
	}
	if got := dockerBinary(nil); got != "docker" {
		t.Fatalf("docker binary = %q, want the docker fallback", got)
	}
}
