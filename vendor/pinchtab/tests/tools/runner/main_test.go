package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRoutesE2E(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"e2e", "--suite", "basic", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "runner e2e (Go) - resolved plan") {
		t.Fatalf("stdout did not contain e2e dry-run plan:\n%s", out)
	}
	if !strings.Contains(out, "docker compose -f tests/e2e/docker-compose.yml up -d pinchtab fixtures") {
		t.Fatalf("stdout did not contain basic shared-stack command:\n%s", out)
	}
}

func TestRunKeepsLegacyBenchFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--provider", "fake", "--lane", "pinchtab", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "resolved plan") {
		t.Fatalf("stdout did not contain benchmark dry-run plan:\n%s", stdout.String())
	}
}
