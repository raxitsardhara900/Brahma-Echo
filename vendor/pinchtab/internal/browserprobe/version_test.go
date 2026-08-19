package browserprobe

import (
	"strconv"
	"strings"
	"testing"
)

// The fallback is what every version surface advertises when the launched binary cannot
// be probed, and it reaches the wire unvalidated: the UA string, the Sec-CH-UA brands and
// fullVersionList are all derived from it. A malformed value is therefore not a config
// error someone sees — it is a browser describing itself in a shape no Chrome ever sends,
// which is louder than the stale version it replaces.
func TestFallbackChromeVersionHasTheShapeChromeActuallySends(t *testing.T) {
	parts := strings.Split(FallbackChromeVersion, ".")
	if len(parts) != 4 {
		t.Fatalf("FallbackChromeVersion = %q has %d components, want the four-part major.minor.build.patch Chrome reports", FallbackChromeVersion, len(parts))
	}
	for i, part := range parts {
		if part == "" {
			t.Fatalf("FallbackChromeVersion = %q has an empty component %d", FallbackChromeVersion, i)
		}
		if _, err := strconv.Atoi(part); err != nil {
			t.Errorf("FallbackChromeVersion component %d = %q is not numeric", i, part)
		}
	}
	if major, _ := strconv.Atoi(parts[0]); major < 100 {
		t.Errorf("FallbackChromeVersion major = %d; Chrome's UA reduction assumes v100+, and every reduced UA derived from this would be a version Chrome never shipped", major)
	}
}

// The fallback and the probe answer the same question, so the probe's own extractor must
// read the fallback back unchanged. If it cannot, the two disagree about what a version
// looks like and a probed value would take a different path through the callers than the
// literal it replaces.
func TestTheProbeExtractorReadsTheFallbackBackUnchanged(t *testing.T) {
	if got := ExtractVersionToken("Google Chrome " + FallbackChromeVersion); got != FallbackChromeVersion {
		t.Errorf("ExtractVersionToken(...%s) = %q, want the fallback read back unchanged", FallbackChromeVersion, got)
	}
	if got := CompareSemver(FallbackChromeVersion, FallbackChromeVersion); got != 0 {
		t.Errorf("CompareSemver(fallback, fallback) = %d, want 0", got)
	}
}
