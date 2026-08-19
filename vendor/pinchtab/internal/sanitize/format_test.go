package sanitize

import (
	"strings"
	"testing"
)

// StripControlChars relies on unicode.IsControl, which only covers category Cc.
// Bidirectional overrides and zero-width blanks are category Cf, so they passed
// through the sanitizer that exists precisely to stop untrusted text rendering
// as something other than its contents.
func TestCleanErrorStripsTextSpoofingFormatRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		gone string
	}{
		{"right-to-left override", "Error: \u202Egnp.exe\u202C failed", "\u202E"},
		{"pop directional formatting", "Error: \u202Egnp.exe\u202C failed", "\u202C"},
		{"left-to-right mark", "admin\u200E", "\u200E"},
		{"first strong isolate", "\u2068spoofed\u2069", "\u2068"},
		{"zero width space", "ad\u200Bmin", "\u200B"},
		{"byte order mark", "tok\uFEFFen", "\uFEFF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanError(tt.in, 200); strings.Contains(got, tt.gone) {
				t.Errorf("CleanError(%q) = %q, still contains %q", tt.in, got, tt.gone)
			}
			if got := CleanForLog(tt.in, 200); strings.Contains(got, tt.gone) {
				t.Errorf("CleanForLog(%q) = %q, still contains %q", tt.in, got, tt.gone)
			}
		})
	}
}

// Zero-width joiners are required to render emoji sequences and Indic/Arabic
// script correctly, so stripping every format character would corrupt
// legitimate text. They must survive.
func TestCleanErrorKeepsJoinersAndOrdinaryText(t *testing.T) {
	for _, in := range []string{
		"family \U0001F468\u200D\U0001F469\u200D\U0001F467 emoji",
		"\u0915\u094D\u200Cष",
		"plain ascii message",
		"unicode: café — naïve",
	} {
		if got := CleanError(in, 200); got != in {
			t.Errorf("CleanError(%q) = %q, want unchanged", in, got)
		}
	}
}
