package stealth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/pinchtab/pinchtab/internal/browserprobe"
	"github.com/pinchtab/pinchtab/internal/browsers/runtimekit"
	"github.com/pinchtab/pinchtab/internal/config"
)

var browserVersionCache = browserprobe.NewVersionCache(browserprobe.RunVersion)

// ResolveBrowserVersion is the Chrome version the persona advertises. An explicit
// browser.version in config wins; otherwise the launched binary is probed once per
// resolved path and that version is advertised. A missing binary or failed probe
// returns "", which the persona builders reduce to browserprobe.FallbackChromeVersion,
// so a probe failure never blocks a launch.
func ResolveBrowserVersion(cfg *config.RuntimeConfig) string {
	return resolveBrowserVersion(browserVersionCache, cfg)
}

func resolveBrowserVersion(cache *browserprobe.VersionCache, cfg *config.RuntimeConfig) string {
	if cfg == nil {
		return ""
	}
	if explicit := strings.TrimSpace(cfg.BrowserVersion); explicit != "" {
		return explicit
	}
	return cache.Version(context.Background(), runtimekit.ResolveEffectiveBrowser(cfg).Binary)
}

// ProbedBinaryVersion is the version the launched binary reports for itself,
// independent of what the persona advertises, so a caller can compare the two. It
// shares the resolver's cache, so a binary is probed at most once per process.
// Empty when no binary resolves or the probe fails.
func ProbedBinaryVersion(cfg *config.RuntimeConfig) string {
	if cfg == nil {
		return ""
	}
	return browserVersionCache.Version(context.Background(), runtimekit.ResolveEffectiveBrowser(cfg).Binary)
}

// WarnBrowserVersionMismatch logs once at launch when an explicit browser.version
// disagrees with the launched binary. The explicit value stays authoritative — a
// deliberate pin is honoured — but the stale-pin population the probe skips is made
// visible and actionable rather than silently overridden.
func WarnBrowserVersionMismatch(cfg *config.RuntimeConfig) {
	if cfg == nil {
		return
	}
	explicit := strings.TrimSpace(cfg.BrowserVersion)
	if explicit == "" {
		return
	}
	probed := ProbedBinaryVersion(cfg)
	if probed == "" || browserprobe.CompareSemver(explicit, probed) == 0 {
		return
	}
	slog.Warn("configured browser.version disagrees with the launched browser binary; the persona advertises the configured value",
		"configured", explicit, "binary", probed, "configKey", "browser.version")
}
