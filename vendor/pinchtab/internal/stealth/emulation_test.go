package stealth

import (
	"context"
	"testing"

	_ "github.com/pinchtab/pinchtab/internal/browsers/chrome"
	_ "github.com/pinchtab/pinchtab/internal/browsers/cloak"
	"github.com/pinchtab/pinchtab/internal/config"
)

// ApplyTargetEmulation must self-defend on native Cloak: it returns nil before
// any CDP emulation override runs, so a background (non-chromedp) context is
// enough — no executor is needed because the guard short-circuits first.
func TestApplyTargetEmulation_SkipsNativeCloak(t *testing.T) {
	cfg := &config.RuntimeConfig{
		DefaultBrowser: config.BrowserCloak,
		Cloak: config.CloakBrowserRuntimeConfig{
			FingerprintSeed:           "42069",
			DisableDefaultStealthArgs: true,
		},
	}
	if !config.PinchTabStealthDefaultsDisabled(cfg) {
		t.Fatal("precondition: cfg should report PinchTab stealth defaults disabled")
	}
	if err := ApplyTargetEmulation(context.Background(), cfg, ""); err != nil {
		t.Fatalf("ApplyTargetEmulation on native cloak should no-op, got %v", err)
	}
}

func TestApplyTargetEmulation_NilConfig(t *testing.T) {
	if err := ApplyTargetEmulation(context.Background(), nil, ""); err != nil {
		t.Fatalf("nil cfg should no-op, got %v", err)
	}
}

// Chrome stops emitting Sec-CH-UA the moment setUserAgentOverride arrives without
// metadata, so attaching it is what keeps a persona's hints on the wire — and omitting it
// for a non-Chromium identity is what keeps them off, which is what real Safari does.
func TestBuildUserAgentOverrideAttachesMetadataOnlyForAChromiumIdentity(t *testing.T) {
	const version = "144.0.7559.133"

	chrome := BuildUserAgentOverride(ChromeUserAgent(PlatformMacOS, ReducedBrowserVersion(version)), version)
	if chrome == nil || chrome.UserAgentMetadata == nil {
		t.Fatal("a Chrome persona carries no userAgentMetadata, so every sec-ch-ua header stops being sent")
	}
	if chrome.UserAgentMetadata.Platform != PlatformMacOS {
		t.Errorf("metadata platform = %q, want %q", chrome.UserAgentMetadata.Platform, PlatformMacOS)
	}

	safari := BuildUserAgentOverride("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15", version)
	if safari == nil {
		t.Fatal("a Safari persona built no override at all; the UA still has to be applied")
	}
	if safari.UserAgentMetadata != nil {
		t.Errorf("a Safari identity advertises %+v; Safari implements no UA-CH, so the hints must stay absent", safari.UserAgentMetadata)
	}
}
