package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestRunNonInteractiveSetupDoesNotPrintToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultFileConfig()
	cfg.Server.Token = "very-secret-token-value"
	cfg.Security.AllowedDomains = []string{"localhost"}

	output := captureStdout(t, func() {
		// tokenGenerated=true: the only case that still writes, and the case whose
		// output this test is about.
		if !runNonInteractiveSetup(&cfg, configPath, true, true) {
			t.Fatal("runNonInteractiveSetup() = false")
		}
	})

	if strings.Contains(output, "very-secret-token-value") {
		t.Fatalf("expected token to stay hidden, got %q", output)
	}
	if strings.Contains(output, "Token:") {
		t.Fatalf("expected setup output to omit token preview, got %q", output)
	}
}

func TestRunUpgradeNoticeDoesNotPrintToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultFileConfig()
	cfg.ConfigVersion = "0.9.0"
	cfg.Server.Token = "very-secret-token-value"

	output := captureStdout(t, func() {
		if !runUpgradeNotice(&cfg, configPath, false) {
			t.Fatal("runUpgradeNotice() = false")
		}
	})

	if strings.Contains(output, "very-secret-token-value") {
		t.Fatalf("expected token to stay hidden, got %q", output)
	}
	if strings.Contains(output, "Token:") {
		t.Fatalf("expected upgrade output to omit token preview, got %q", output)
	}
}

func TestApplyPostureGuardUp(t *testing.T) {
	cfg := &config.FileConfig{}
	cfg.Server.Bind = "0.0.0.0" // sentinel: Guard UP must overwrite the bind

	applyPosture(cfg, guardUpPosture)

	allows := map[string]*bool{
		"AllowEvaluate":   cfg.Security.AllowEvaluate,
		"AllowDownload":   cfg.Security.AllowDownload,
		"AllowCookies":    cfg.Security.AllowCookies,
		"AllowUpload":     cfg.Security.AllowUpload,
		"AllowMacro":      cfg.Security.AllowMacro,
		"AllowScreencast": cfg.Security.AllowScreencast,
	}
	for name, p := range allows {
		if p == nil || *p != false {
			t.Errorf("Guard UP %s = %v, want non-nil false", name, p)
		}
	}
	if !cfg.Security.IDPI.Enabled || !cfg.Security.IDPI.StrictMode ||
		!cfg.Security.IDPI.ScanContent || !cfg.Security.IDPI.WrapContent {
		t.Errorf("Guard UP IDPI = %+v, want all enabled", cfg.Security.IDPI)
	}
	got := cfg.Security.AllowedDomains
	if len(got) != 3 || got[0] != "127.0.0.1" || got[1] != "localhost" || got[2] != "::1" {
		t.Errorf("Guard UP AllowedDomains = %v, want loopback trio", got)
	}
	if cfg.Server.Bind != "127.0.0.1" {
		t.Errorf("Guard UP Bind = %q, want 127.0.0.1", cfg.Server.Bind)
	}
}

func TestApplyPostureGuardDown(t *testing.T) {
	cfg := &config.FileConfig{}
	cfg.Server.Bind = "203.0.113.5" // sentinel: Guard DOWN must leave the bind untouched

	applyPosture(cfg, guardDownPosture)

	allows := map[string]*bool{
		"AllowEvaluate":   cfg.Security.AllowEvaluate,
		"AllowDownload":   cfg.Security.AllowDownload,
		"AllowCookies":    cfg.Security.AllowCookies,
		"AllowUpload":     cfg.Security.AllowUpload,
		"AllowMacro":      cfg.Security.AllowMacro,
		"AllowScreencast": cfg.Security.AllowScreencast,
	}
	for name, p := range allows {
		if p == nil || *p != true {
			t.Errorf("Guard DOWN %s = %v, want non-nil true", name, p)
		}
	}
	if cfg.Security.IDPI.Enabled || cfg.Security.IDPI.StrictMode ||
		cfg.Security.IDPI.ScanContent || cfg.Security.IDPI.WrapContent {
		t.Errorf("Guard DOWN IDPI = %+v, want all disabled", cfg.Security.IDPI)
	}
	if len(cfg.Security.AllowedDomains) != 0 {
		t.Errorf("Guard DOWN AllowedDomains = %v, want empty", cfg.Security.AllowedDomains)
	}
	if cfg.Server.Bind != "203.0.113.5" {
		t.Errorf("Guard DOWN Bind = %q, want the sentinel left unchanged", cfg.Server.Bind)
	}
}

// The printed summary must reflect the same posture struct the config mutator
// uses, so the wizard cannot promise a posture it does not persist.
func TestPrintPostureReflectsPosture(t *testing.T) {
	up := captureStdout(t, func() { printPosture(guardUpPosture) })
	for _, want := range []string{"127.0.0.1, localhost, ::1", "disabled", "strict"} {
		if !strings.Contains(up, want) {
			t.Errorf("Guard UP summary missing %q, got:\n%s", want, up)
		}
	}
	if strings.Contains(up, "enabled") {
		t.Errorf("Guard UP summary should not advertise an enabled feature, got:\n%s", up)
	}

	down := captureStdout(t, func() { printPosture(guardDownPosture) })
	for _, want := range []string{"all", "enabled", "off"} {
		if !strings.Contains(down, want) {
			t.Errorf("Guard DOWN summary missing %q, got:\n%s", want, down)
		}
	}
	if strings.Contains(down, "strict") {
		t.Errorf("Guard DOWN summary should not advertise IDPI strict, got:\n%s", down)
	}
}

// The startup write announces itself and reports its failure. Both halves were silent:
// nothing said a plain `pinchtab server` had touched the user's file, and the save error
// was discarded with `_ =`, so a config the user had marked read-only appeared to be
// rewritten successfully.
func TestRecordConfigVersionAnnouncesItselfAndReportsAFailedWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only file mode semantics")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	cfg, _, err := config.LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}

	announced := captureStderr(t, func() {
		if !recordConfigVersion(cfg, path, true, false) {
			t.Error("recordConfigVersion() = false for a writable config")
		}
	})
	if !strings.Contains(announced, "recording configVersion") || !strings.Contains(announced, path) {
		t.Errorf("the startup write did not announce itself on stderr, got %q", announced)
	}

	// A separate file for the read-only half: the first call already stamped the one
	// above, and a save with nothing left to write is skipped rather than refused —
	// which is its own correct form of leaving a protected file alone, but proves
	// nothing about reporting.
	locked := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(locked, []byte(`{"server":{"port":"9913","token":"tok3"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", locked)
	lockedCfg, _, err := config.LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0444); err != nil {
		t.Fatal(err)
	}
	reported := captureStderr(t, func() {
		if recordConfigVersion(lockedCfg, locked, false, false) {
			t.Error("recordConfigVersion() = true against a read-only config; the failure must not be swallowed")
		}
	})
	if !strings.Contains(reported, "could not record configVersion") {
		t.Errorf("a refused write was not reported on stderr, got %q", reported)
	}
	if fi, err := os.Stat(locked); err != nil {
		t.Fatal(err)
	} else if got := fi.Mode().Perm(); got != 0444 {
		t.Errorf("mode after a refused write = %o, want 0444", got)
	}
}
