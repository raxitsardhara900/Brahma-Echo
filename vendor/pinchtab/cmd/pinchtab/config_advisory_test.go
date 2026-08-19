package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

// writeConfigCarryingTheIgnoredKey plants a config whose file holds the inert
// observability.activity.stateDir key, which is the state a user reaches by having set it
// before it was refused.
func writeConfigCarryingTheIgnoredKey(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	fc := config.DefaultFileConfig()
	fc.Server.Port = "9867"
	fc.Observability.Activity.StateDir = "/tmp/elsewhere"
	if err := config.SaveFileConfig(&fc, path); err != nil {
		t.Fatalf("SaveFileConfig() error = %v", err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	saved, _, err := config.LoadFileConfig()
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if saved.Observability.Activity.StateDir == "" {
		t.Fatal("the fixture lost the ignored key, so this test would prove nothing")
	}
	return path
}

// This is the agent shape and the only one that shows the bug: off a TTY confirmSaveAnyway
// cannot ask, so it answers no, and while the ignored key rode the gating list every later
// config write — to any key — was aborted. A TTY-attached run would prompt and pass.
func TestConfigSetSucceedsOffATTYWithTheIgnoredKeyInTheFile(t *testing.T) {
	if isInteractiveTerminal() {
		t.Skip("this case only exists off a TTY; the test binary is attached to one")
	}
	writeConfigCarryingTheIgnoredKey(t)

	var err error
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err = handleConfigSet("server.port", "9999")
		})
	})
	if err != nil {
		t.Fatalf("config set error = %v, want the write to succeed; an inert key must not gate unrelated writes", err)
	}
	if !strings.Contains(stderr, config.ActivityStateDirAdvisory) {
		t.Errorf("stderr = %q, want the advisory reported rather than swallowed", stderr)
	}

	saved, _, loadErr := config.LoadFileConfig()
	if loadErr != nil {
		t.Fatalf("LoadFileConfig() error = %v", loadErr)
	}
	if saved.Server.Port != "9999" {
		t.Errorf("server.port = %q, want the value the write asked for", saved.Server.Port)
	}
	if saved.Observability.Activity.StateDir == "" {
		t.Error("the write cleared the ignored key; this fix reports it, it does not rewrite the user's file")
	}
}

// The guardrail: the fix must not turn a skipped save into a success. A genuinely invalid
// value still aborts off a TTY, non-zero, with the file untouched.
func TestConfigSetStillAbortsOffATTYOnARealValidationError(t *testing.T) {
	if isInteractiveTerminal() {
		t.Skip("this case only exists off a TTY; the test binary is attached to one")
	}
	path := writeConfigCarryingTheIgnoredKey(t)
	before, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}

	var err error
	_ = captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err = handleConfigSet("server.port", "99999")
		})
	})
	if err == nil {
		t.Fatal("an out-of-range port saved without complaint; the gate must still hold for real errors")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("error = %v, want the abort that makes the CLI exit non-zero", err)
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Error("the aborted write changed the file on disk")
	}
}

// Setting the ignored key itself is still refused on that write — the severity split is
// what keeps carrying it advisory and setting it an error.
func TestConfigSetOfTheIgnoredKeyIsStillRefused(t *testing.T) {
	writeConfigCarryingTheIgnoredKey(t)

	err := handleConfigSet("observability.activity.stateDir", "/tmp/elsewhere-again")
	if err == nil {
		t.Fatal("config set observability.activity.stateDir was accepted")
	}
	if err.Error() != config.ActivityStateDirRefusal {
		t.Errorf("refusal = %q, want %q", err, config.ActivityStateDirRefusal)
	}
}
