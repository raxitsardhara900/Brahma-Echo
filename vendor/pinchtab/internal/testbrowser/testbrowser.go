// Package testbrowser resolves a launchable browser for browser-backed tests
// the same way PinchTab resolves one at runtime. Tests used to PATH-probe for a
// binary named "chromium", which finds nothing on a stock macOS even with Chrome
// installed, so every browser-backed test silently skipped on the primary
// development platform.
package testbrowser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/browsers"
	_ "github.com/pinchtab/pinchtab/internal/browsers/chrome"
	_ "github.com/pinchtab/pinchtab/internal/browsers/cloak"
	_ "github.com/pinchtab/pinchtab/internal/browsers/ghostchrome"
	"github.com/pinchtab/pinchtab/internal/browsers/runtimekit"
)

// BinaryEnv points the browser-backed tests at a specific binary.
const BinaryEnv = "PINCHTAB_TEST_BROWSER"

// SkipEnv opts out of browser-backed tests entirely, for a lightweight run.
const SkipEnv = "PINCHTAB_SKIP_BROWSER_TESTS"

// preferredIDs is the order providers are tried in: chrome first because these
// tests assert plain Chromium behaviour, then whatever else is registered.
var preferredIDs = []string{"chrome", "cloak", "ghost-chrome"}

// Find returns a launchable browser binary and the locations that were probed to
// reach that answer. The binary is empty when discovery found nothing. Discovery
// is runtimekit's — the same call the server makes when it launches a browser.
func Find() (binary string, probed []string) {
	if override := strings.TrimSpace(os.Getenv(BinaryEnv)); override != "" {
		return override, []string{BinaryEnv + "=" + override}
	}
	for _, id := range providerIDs() {
		if found := strings.TrimSpace(runtimekit.FindBrowserBinary(id)); found != "" {
			return found, probed
		}
		probed = append(probed, probedPathsFor(id)...)
	}
	return "", probed
}

func probedPathsFor(id string) []string {
	b, ok := browsers.Get(id)
	if !ok {
		return nil
	}
	return b.DiscoverBinary().Probed
}

func providerIDs() []string {
	ordered := append([]string(nil), preferredIDs...)
	seen := map[string]bool{}
	for _, id := range ordered {
		seen[id] = true
	}
	for _, id := range browsers.IDs() {
		if !seen[id] {
			ordered = append(ordered, id)
		}
	}
	return ordered
}

// Path returns a launchable browser binary, skipping the test only when the
// machine genuinely has none. The skip names what was searched and how to point
// the tests at a binary, so a real absence is actionable.
func Path(t testing.TB) string {
	t.Helper()
	if skipRequested() {
		t.Skipf("browser-backed test skipped: %s is set", SkipEnv)
	}
	binary, probed := Find()
	if binary == "" {
		t.Skip(notFoundMessage(probed))
	}
	return binary
}

// MustPath is Path for tests a contributor has explicitly opted into: once the
// opt-in is given, a missing browser is a failure rather than a silent skip.
func MustPath(t testing.TB) string {
	t.Helper()
	binary, probed := Find()
	if binary == "" {
		t.Fatal(notFoundMessage(probed))
	}
	return binary
}

func skipRequested() bool {
	v := strings.TrimSpace(os.Getenv(SkipEnv))
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

func notFoundMessage(probed []string) string {
	searched := "nothing"
	if len(probed) > 0 {
		searched = strings.Join(probed, ", ")
	}
	return fmt.Sprintf(
		"no browser binary found for %s (searched: %s); set %s=/path/to/browser to point the browser-backed tests at one",
		strings.Join(providerIDs(), "/"), searched, BinaryEnv,
	)
}
