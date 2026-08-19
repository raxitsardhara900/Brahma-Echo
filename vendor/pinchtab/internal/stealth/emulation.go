package stealth

import (
	"context"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/emulation"
	"github.com/pinchtab/pinchtab/internal/config"
)

// BuildUserAgentOverride creates a SetUserAgentOverride action with persona-backed
// metadata. chromeVersion should be the full four-part build, the shape
// browserprobe.FallbackChromeVersion has. If chromeVersion is empty, returns nil.
func BuildUserAgentOverride(userAgent, chromeVersion string) *emulation.SetUserAgentOverrideParams {
	if chromeVersion == "" {
		return nil
	}

	persona := BuildPersona(userAgent, chromeVersion)
	if persona.UserAgent == "" {
		return nil
	}

	override := emulation.SetUserAgentOverride(persona.UserAgent).
		WithAcceptLanguage(persona.AcceptLanguage).
		WithPlatform(persona.NavigatorPlatform)
	if metadata := personaUserAgentMetadata(persona); metadata != nil {
		override = override.WithUserAgentMetadata(metadata)
	}
	return override
}

// personaUserAgentMetadata is the UA-CH half of a persona, or nil for a persona that
// claims no Chromium brand. Nil is the honest answer there rather than an omission:
// Chrome stops emitting Sec-CH-UA when setUserAgentOverride carries no metadata, and a
// non-Chromium identity sends none of those headers either.
func personaUserAgentMetadata(persona BrowserPersona) *emulation.UserAgentMetadata {
	if len(persona.UserAgentData.Brands) == 0 {
		return nil
	}
	return &emulation.UserAgentMetadata{
		Platform:        persona.UserAgentData.Platform,
		PlatformVersion: persona.UserAgentData.PlatformVersion,
		Architecture:    persona.UserAgentData.Architecture,
		Bitness:         persona.UserAgentData.Bitness,
		Mobile:          persona.UserAgentData.Mobile,
		Brands:          brandVersions(persona.UserAgentData.Brands),
		FullVersionList: brandVersions(persona.UserAgentData.FullVersionList),
	}
}

func brandVersions(brands []BrandVersion) []*emulation.UserAgentBrandVersion {
	converted := make([]*emulation.UserAgentBrandVersion, 0, len(brands))
	for _, brand := range brands {
		converted = append(converted, &emulation.UserAgentBrandVersion{
			Brand:   brand.Brand,
			Version: brand.Version,
		})
	}
	return converted
}

func BuildLocaleOverride(userAgent, chromeVersion string) *emulation.SetLocaleOverrideParams {
	persona := BuildPersona(userAgent, chromeVersion)
	if persona.Language == "" {
		return nil
	}
	return emulation.SetLocaleOverride().WithLocale(persona.Language)
}

// ApplyTargetEmulation applies the launch persona to a page/worker target so
// navigator.*, client hints, Accept-Language, and Intl locale stay coherent.
func ApplyTargetEmulation(ctx context.Context, cfg *config.RuntimeConfig, userAgent string) error {
	if cfg == nil {
		return nil
	}

	if config.PinchTabStealthDefaultsDisabled(cfg) {
		// Native Cloak does its own source-level fingerprinting; PinchTab's
		// page/worker emulation overrides would conflict. Self-defend in case a
		// future caller forgets to guard (all current callers already do).
		return nil
	}

	if err := emulation.SetAutomationOverride(false).Do(ctx); err != nil {
		return fmt.Errorf("automation override: %w", err)
	}

	if localeOverride := BuildLocaleOverride(userAgent, ResolveBrowserVersion(cfg)); localeOverride != nil {
		if err := localeOverride.Do(ctx); err != nil {
			return fmt.Errorf("locale override: %w", err)
		}
	}

	// Override the UA Client Hints metadata ONLY when the caller supplied an
	// explicit UA (userAgent is the launch --user-agent, set only for a configured
	// custom UA). Otherwise defer to Chrome's NATIVE UA-CH: the synthesized
	// metadata is inconsistent with a real Chrome — a stale GREASE brand and
	// (because cdproto's UserAgentMetadata has no full_version field) an empty
	// uaFullVersion — whereas the native hints are correct and self-consistent.
	if strings.TrimSpace(userAgent) != "" {
		if uaOverride := BuildUserAgentOverride(userAgent, ResolveBrowserVersion(cfg)); uaOverride != nil {
			if err := uaOverride.Do(ctx); err != nil {
				return fmt.Errorf("user agent override: %w", err)
			}
		}
	}

	return nil
}
