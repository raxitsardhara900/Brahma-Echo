package main

import (
	"bytes"
	"strings"
	"testing"
)

// The gate rejection suppresses usage at its return site, not at declaration,
// so a genuine misuse must still get the usage block.
func TestCompareWrongArgumentCountStillPrintsUsage(t *testing.T) {
	var out bytes.Buffer
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		compareCmd.SilenceUsage = false
	})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"compare", "http://only-one-url"})

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("compare with one argument should be rejected")
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("argument error lost its usage block: %q", out.String())
	}
}

func TestCompareHelpNamesUncaughtJSErrors(t *testing.T) {
	if !strings.Contains(compareCmd.Long, "Uncaught JS errors") {
		t.Fatalf("compare help does not name uncaught JS errors: %q", compareCmd.Long)
	}
}
