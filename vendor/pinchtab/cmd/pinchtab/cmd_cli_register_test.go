package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// Guard for the registration refactor: every browser root command must be in
// the "browser" group, and the shared pointer-flag bundle (+ per-command extras)
// must survive the helper extraction.
func TestBrowserCommandRegistration(t *testing.T) {
	for _, c := range browserRootCommands() {
		if c.GroupID != "browser" {
			t.Errorf("command %q GroupID = %q, want %q", c.Name(), c.GroupID, "browser")
		}
	}

	for _, name := range []string{"css", "x", "y", "humanize", "wait-nav", "mode", "submit", "dismiss-known-interstitials"} {
		if clickCmd.Flags().Lookup(name) == nil {
			t.Errorf("clickCmd missing flag %q", name)
		}
	}
	// Pointer commands keep their action-specific extras alongside the bundle.
	if mouseDownCmd.Flags().Lookup("button") == nil {
		t.Error("mouseDownCmd missing button flag")
	}
	for _, name := range []string{"css", "x", "y", "humanize"} {
		if hoverCmd.Flags().Lookup(name) == nil {
			t.Errorf("hoverCmd missing flag %q", name)
		}
	}
}

func TestCaptureCommandRegistersTabFlag(t *testing.T) {
	if captureCmd.Flags().Lookup("tab") == nil {
		t.Fatal("captureCmd missing --tab flag")
	}
}

func TestNavigateCommandRegistersTimeoutInSeconds(t *testing.T) {
	flag := navCmd.Flags().Lookup("timeout")
	if flag == nil {
		t.Fatal("navCmd missing --timeout flag")
	}
	if flag.DefValue != "0" || flag.Usage != "Navigation timeout in seconds (max 120); overrides the 30s new-tab ceiling" {
		t.Fatalf("--timeout default/usage = %q / %q", flag.DefValue, flag.Usage)
	}
}

// TestPostActionFlagsBundle pins the exact usage strings the shared
// addPostActionFlags helper interpolates per verb, so a future verb edit cannot
// silently drift the --help text, and verifies the one no-text command omits it.
func TestPostActionFlagsBundle(t *testing.T) {
	wantUsage := func(cmd *cobra.Command, flag, want string) {
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Errorf("%s missing flag %q", cmd.Name(), flag)
			return
		}
		if f.Usage != want {
			t.Errorf("%s --%s usage = %q, want %q", cmd.Name(), flag, f.Usage, want)
		}
	}

	wantUsage(clickCmd, "snap", "Output interactive snapshot after action")
	wantUsage(clickCmd, "snap-diff", "Output snapshot diff after action (changes only)")
	wantUsage(clickCmd, "text", "Output page text after action (for verification)")
	wantUsage(clickCmd, "submit", "Dispatch one DOM click and report bounded post-submit state")

	wantUsage(reloadCmd, "snap", "Output interactive snapshot after reload")
	wantUsage(reloadCmd, "snap-diff", "Output snapshot diff after reload (changes only)")
	wantUsage(reloadCmd, "text", "Output page text after reload (for verification)")

	wantUsage(navCmd, "snap", "Output interactive snapshot after navigation")
	wantUsage(navCmd, "snap-diff", "Output snapshot diff after navigation (changes only)")
	// nav has --text because landing on a page is exactly when reading it is
	// useful, and reload — which lands on a page the same way — always had it.
	wantUsage(navCmd, "text", "Output page text after navigation (for verification)")

	// scroll stays excluded: scrolling does not change which document is loaded,
	// so post-scroll text answers nothing --snap-diff does not answer better.
	if scrollCmd.Flags().Lookup("text") != nil {
		t.Error("scrollCmd should not register a post-action --text flag")
	}
}

// --css-1x was removed in favor of --scale; it must remain registered as a
// hidden, deprecated no-op so old scripts get a notice instead of a hard
// "unknown flag" error.
func TestScreenshotCSS1xDeprecatedShim(t *testing.T) {
	f := screenshotCmd.Flags().Lookup("css-1x")
	if f == nil {
		t.Fatal("css-1x flag should still be registered as a deprecated shim (else old scripts hard-error)")
	}
	if f.Deprecated == "" {
		t.Error("css-1x should be marked deprecated")
	}
	if !f.Hidden {
		t.Error("deprecated css-1x should be hidden from --help")
	}
}
