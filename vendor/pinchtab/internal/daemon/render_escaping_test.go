package daemon

import (
	"encoding/xml"
	"strings"
	"testing"
)

// The plist is XML, and the paths interpolated into it are user-influenced —
// --config / PINCHTAB_CONFIG in particular. Directory names containing "&" are
// ordinary on macOS ("R&D"), and an unescaped one makes launchctl reject the
// file with an XML parse error rather than anything actionable.
func TestRenderLaunchdPlistStaysWellFormedXML(t *testing.T) {
	paths := []struct {
		name string
		path string
	}{
		{"ampersand", "/Users/tester/R&D/pinchtab.json"},
		{"angle brackets", "/Users/tester/a<b>c/pinchtab.json"},
		{"quotes", `/Users/tester/say "hi"/pinchtab.json`},
	}

	for _, tt := range paths {
		t.Run(tt.name, func(t *testing.T) {
			plist := renderLaunchdPlist("/usr/local/bin/pinchtab", tt.path, "/Users/tester",
				"/Users/tester/.pinchtab/logs/out.log", "/Users/tester/.pinchtab/logs/err.log")

			decoder := xml.NewDecoder(strings.NewReader(plist))
			for {
				_, err := decoder.Token()
				if err != nil {
					if err.Error() == "EOF" {
						break
					}
					t.Fatalf("plist is not well-formed XML with config path %q: %v", tt.path, err)
				}
			}
		})
	}
}

// The escaped value must still round-trip to the original path, or the daemon
// would launch with a corrupted config location.
func TestRenderLaunchdPlistPreservesPathValue(t *testing.T) {
	const configPath = "/Users/tester/R&D/pinchtab.json"
	plist := renderLaunchdPlist("/usr/local/bin/pinchtab", configPath, "/Users/tester", "/o.log", "/e.log")

	var doc struct {
		Strings []string `xml:"dict>dict>string"`
	}
	if err := xml.Unmarshal([]byte(plist), &doc); err != nil {
		t.Fatalf("unmarshal plist: %v", err)
	}
	var found bool
	for _, s := range doc.Strings {
		if s == configPath {
			found = true
		}
	}
	if !found {
		t.Errorf("config path did not round-trip through the plist, got %q", doc.Strings)
	}
}

// systemd expands "%" specifiers in unit files (%h, %i, ...), so a literal
// percent in a path must be doubled or the daemon starts with a mangled
// config location.
func TestRenderSystemdUnitEscapesPercent(t *testing.T) {
	const configPath = "/home/tester/100%backup/pinchtab.json"
	unit := renderSystemdUnit("/usr/local/bin/pinchtab", configPath, "/o.log", "/e.log")

	if strings.Contains(unit, "100%backup") {
		t.Errorf("literal %% reached the unit file unescaped: %s", unit)
	}
	if !strings.Contains(unit, "100%%backup") {
		t.Errorf("expected the percent to be doubled, got: %s", unit)
	}
}
