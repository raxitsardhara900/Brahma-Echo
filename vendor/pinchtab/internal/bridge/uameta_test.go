package bridge

import (
	"strings"
	"testing"
)

func TestBuild_Empty(t *testing.T) {
	if buildUserAgentOverride("", "") != nil {
		t.Fatal("expected nil for empty chrome version")
		return
	}

	p := buildUserAgentOverride("", "144.0.0.0")
	if p == nil {
		t.Fatal("expected non-nil for empty user agent with chromeVersion")
		return
	}
	if p.UserAgent == "" {
		t.Fatal("expected generated user agent")
		return
	}
}

func TestBuild_UsesResolvedUserAgent(t *testing.T) {
	p := buildUserAgentOverride("", "144.0.0.0")
	if p == nil {
		t.Fatal("expected non-nil")
		return
	}
	if !strings.Contains(p.UserAgent, "Chrome/144.0.0.0") {
		t.Fatalf("expected resolved UA to contain full Chrome version, got %q", p.UserAgent)
		return
	}
}

func TestBuild_Versions(t *testing.T) {
	p := buildUserAgentOverride(chromeUserAgentFixture, "144.0.7559.133")
	if p == nil {
		t.Fatal("expected non-nil")
		return
	}
	meta := p.UserAgentMetadata
	if meta == nil {
		t.Fatal("expected metadata")
		return
	}
	for _, b := range meta.Brands {
		if b.Brand == "Google Chrome" && b.Version != "144" {
			t.Errorf("expected major version 144, got %s", b.Version)
		}
	}
	for _, b := range meta.FullVersionList {
		if b.Brand == "Google Chrome" && b.Version != "144.0.7559.133" {
			t.Errorf("expected full version 144.0.7559.133, got %s", b.Version)
		}
	}
}

func TestBuild_UsesPersonaMetadata(t *testing.T) {
	p := buildUserAgentOverride("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.7559.133 Safari/537.36", "144.0.7559.133")
	if p == nil || p.UserAgentMetadata == nil {
		t.Fatal("expected metadata")
		return
	}
	if p.AcceptLanguage != "en-US,en" {
		t.Fatalf("expected accept language from persona, got %q", p.AcceptLanguage)
		return
	}
	if p.Platform != "Win32" {
		t.Fatalf("expected navigator platform Win32, got %q", p.Platform)
		return
	}
	if got := p.UserAgentMetadata.Platform; got != "Windows" {
		t.Fatalf("expected UA data platform Windows, got %q", got)
		return
	}
}

func TestBuildLocaleOverride_UsesPersonaLanguage(t *testing.T) {
	p := buildLocaleOverride("", "144.0.7559.133")
	if p == nil {
		t.Fatal("expected locale override")
		return
	}
	if p.Locale != "en-US" {
		t.Fatalf("expected locale override en-US, got %q", p.Locale)
		return
	}
}

const (
	chromeUserAgentFixture = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"
	edgeUserAgentFixture   = chromeUserAgentFixture + " Edg/144.0.0.0"
	safariUserAgentFixture = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
)

// The rotate path reaches CDP through SetUserAgentOverride, which built the override by
// hand and passed no metadata — so every Sec-CH-UA header stopped being sent for the life
// of the tab, and the UA it did set had nothing backing it.
func TestUserAgentOverrideActionCarriesClientHintMetadata(t *testing.T) {
	params := UserAgentOverrideParams{UserAgent: edgeUserAgentFixture, Platform: "Win32", AcceptLanguage: "en-GB"}
	override := userAgentOverrideAction(params, "144.0.7559.133")

	if override.UserAgentMetadata == nil {
		t.Fatal("rotate sends no userAgentMetadata, so Chrome drops sec-ch-ua, sec-ch-ua-mobile and sec-ch-ua-platform entirely")
	}
	if got := override.UserAgentMetadata.Platform; got != "Windows" {
		t.Errorf("metadata platform = %q, want Windows to agree with the UA", got)
	}
	if override.Platform != params.Platform {
		t.Errorf("navigator platform = %q, want the caller's %q", override.Platform, params.Platform)
	}
	if override.AcceptLanguage != params.AcceptLanguage {
		t.Errorf("accept-language = %q, want the caller's %q; the persona's own value must not win", override.AcceptLanguage, params.AcceptLanguage)
	}
}

// A UA claiming Edge with Google Chrome brands is a browser that cannot exist, which is
// more detectable than either half alone.
func TestUserAgentOverrideActionBrandsAgreeWithTheUserAgent(t *testing.T) {
	for _, tc := range []struct {
		name      string
		userAgent string
		want      string
		absent    string
	}{
		{name: "edge", userAgent: edgeUserAgentFixture, want: "Microsoft Edge", absent: "Google Chrome"},
		{name: "chrome", userAgent: chromeUserAgentFixture, want: "Google Chrome", absent: "Microsoft Edge"},
	} {
		override := userAgentOverrideAction(UserAgentOverrideParams{UserAgent: tc.userAgent, Platform: "Win32"}, "144.0.7559.133")
		if override.UserAgentMetadata == nil {
			t.Fatalf("%s: no metadata", tc.name)
		}
		brands := map[string]string{}
		for _, brand := range override.UserAgentMetadata.Brands {
			brands[brand.Brand] = brand.Version
		}
		if _, ok := brands[tc.want]; !ok {
			t.Errorf("%s: brands = %v, want %q, which is what a detector cross-checks against the UA string", tc.name, brands, tc.want)
		}
		if _, ok := brands[tc.absent]; ok {
			t.Errorf("%s: brands = %v, want %q absent; the UA and the hints must describe one browser", tc.name, brands, tc.absent)
		}
		if _, ok := brands["Chromium"]; !ok {
			t.Errorf("%s: brands = %v, want the Chromium brand every Chromium build sends", tc.name, brands)
		}
	}
}

// Real Safari sends no Sec-CH-UA at all, so metadata there would be the tell rather than
// the cover — the absence is the coherent answer, not an omission.
func TestUserAgentOverrideActionSendsNoMetadataForANonChromiumIdentity(t *testing.T) {
	override := userAgentOverrideAction(UserAgentOverrideParams{UserAgent: safariUserAgentFixture, Platform: "MacIntel"}, "144.0.7559.133")

	if override.UserAgentMetadata != nil {
		t.Errorf("safari identity carries %+v; a Safari UA advertising Chromium brands is the mismatch this guards against", override.UserAgentMetadata)
	}
	if override.UserAgent != safariUserAgentFixture {
		t.Errorf("UserAgent = %q, want the caller's Safari UA", override.UserAgent)
	}
}
