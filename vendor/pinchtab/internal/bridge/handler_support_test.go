package bridge

import (
	"context"
	"strings"
	"testing"
)

// An empty UserAgent is not "leave it alone" at the CDP layer — Emulation applies
// it, and the tab then advertises an empty navigator.userAgent and sends a blank
// User-Agent header. The rotate handler wrote this field through without a non-empty
// check while guarding every other one, so the refusal belongs in the exported
// BridgeAPI implementation.
//
// Both halves matter. The empty case must be refused BEFORE any CDP call, which is
// why a background context suffices; the non-empty case must get past the guard and
// fail on the context instead, which is what proves the guard is refusing the value
// rather than everything.
func TestSetUserAgentOverrideRefusesAnEmptyUserAgent(t *testing.T) {
	b := &Bridge{}

	for _, empty := range []string{"", "   "} {
		err := b.SetUserAgentOverride(context.Background(), UserAgentOverrideParams{UserAgent: empty, Platform: "Win32"})
		if err == nil {
			t.Fatalf("SetUserAgentOverride(%q) = nil; an empty override blanks navigator.userAgent instead of leaving it alone", empty)
		}
		if !strings.Contains(err.Error(), "user agent") {
			t.Errorf("SetUserAgentOverride(%q) failed with %v, which does not name the argument it refused — the guard is gone and this is the CDP layer failing later", empty, err)
		}
	}

	err := b.SetUserAgentOverride(context.Background(), UserAgentOverrideParams{UserAgent: "Mozilla/5.0 probe", Platform: "Win32"})
	if err == nil {
		t.Fatal("a non-empty override on a bare context reached CDP and succeeded, so this test cannot tell the guard from the transport")
	}
	if strings.Contains(err.Error(), "user agent must not be empty") {
		t.Errorf("a non-empty override was refused by the empty-value guard: %v", err)
	}
}
