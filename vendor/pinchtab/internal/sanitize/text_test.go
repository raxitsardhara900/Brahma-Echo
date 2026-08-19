package sanitize

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCleanErrorRedactsAbsolutePaths(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short unix path",
			input: "/var/log",
			want:  "[path]",
		},
		{
			name:  "quoted unix path",
			input: `error at "/Users/test/file.txt" failed`,
			want:  `error at "[path]" failed`,
		},
		{
			name:  "mixed unix and windows paths",
			input: `copy /var/log to C:\Users\test\file.txt`,
			want:  `copy [path] to [path]`,
		},
		{
			name:  "colon before unix path",
			input: `error:/Users/test/file.txt`,
			want:  `error:[path]`,
		},
		{
			name:  "colon before windows path",
			input: `error:C:\Users\test\file.txt`,
			want:  `error:[path]`,
		},
		{
			name:  "path-like substring inside word is preserved",
			input: `description/Users/guide`,
			want:  `description/Users/guide`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanError(tt.input, 1024); got != tt.want {
				t.Fatalf("CleanError(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// The mirror of the Prefix table below. Carrying the marker is the only thing
// that separates these two exported helpers, and it is the reason callers pick
// one over the other, so it is asserted where the rule is owned rather than left
// to whichever downstream package happens to notice.
func TestTruncateUTF8BytesWithEllipsisCutsOnRuneBoundaryWithMarker(t *testing.T) {
	const s = "héllo"

	tests := []struct {
		name     string
		input    string
		maxBytes int
		want     string
	}{
		{name: "empty input", input: "", maxBytes: 8, want: ""},
		{name: "whole string fits exactly", input: s, maxBytes: len(s), want: s},
		{name: "budget beyond the string", input: s, maxBytes: len(s) + 10, want: s},
		{name: "over the limit gains the marker", input: s, maxBytes: 4, want: "h" + TruncationSuffix},
		{name: "marker budget lands inside a two-byte rune", input: s, maxBytes: 5, want: "h" + TruncationSuffix},
		{name: "budget is exactly the marker", input: s, maxBytes: len(TruncationSuffix), want: TruncationSuffix},
		{name: "budget below the marker keeps what fits", input: s, maxBytes: 1, want: "."},
		{name: "zero budget", input: s, maxBytes: 0, want: ""},
		{name: "negative budget", input: s, maxBytes: -1, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateUTF8BytesWithEllipsis(tt.input, tt.maxBytes)
			if got != tt.want {
				t.Fatalf("TruncateUTF8BytesWithEllipsis(%q, %d) = %q, want %q", tt.input, tt.maxBytes, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("TruncateUTF8BytesWithEllipsis(%q, %d) is not valid UTF-8", tt.input, tt.maxBytes)
			}
			if len(got) > tt.maxBytes && tt.maxBytes > 0 {
				t.Fatalf("TruncateUTF8BytesWithEllipsis(%q, %d) = %q exceeds the byte budget", tt.input, tt.maxBytes, got)
			}
			// Below the marker's own length there is no room for it, so the
			// output is a prefix of the marker rather than content plus marker.
			if len(tt.input) > tt.maxBytes && tt.maxBytes > len(TruncationSuffix) && !strings.Contains(got, TruncationSuffix) {
				t.Fatalf("TruncateUTF8BytesWithEllipsis(%q, %d) = %q cut silently; the marker is what distinguishes it from TruncateUTF8BytesExact", tt.input, tt.maxBytes, got)
			}
		})
	}
}

func TestTruncateUTF8BytesExactCutsOnRuneBoundaryWithoutMarker(t *testing.T) {
	const s = "héllo"

	tests := []struct {
		name     string
		maxBytes int
		want     string
	}{
		{name: "whole string fits", maxBytes: len(s), want: s},
		{name: "budget beyond the string", maxBytes: len(s) + 10, want: s},
		{name: "boundary before a two-byte rune", maxBytes: 1, want: "h"},
		{name: "budget lands inside a two-byte rune", maxBytes: 2, want: "h"},
		{name: "budget lands after a two-byte rune", maxBytes: 3, want: "hé"},
		{name: "zero budget", maxBytes: 0, want: ""},
		{name: "negative budget", maxBytes: -1, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateUTF8BytesExact(s, tt.maxBytes)
			if got != tt.want {
				t.Fatalf("TruncateUTF8BytesExact(%q, %d) = %q, want %q", s, tt.maxBytes, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("TruncateUTF8BytesExact(%q, %d) is not valid UTF-8", s, tt.maxBytes)
			}
			if strings.Contains(got, TruncationSuffix) {
				t.Fatalf("TruncateUTF8BytesExact appended a truncation marker: %q", got)
			}
		})
	}
}

// The helper's own small-budget branch returns TruncationSuffix[:maxBytes] — a raw byte
// slice of a constant, which is the very rule this package exists to enforce. It is safe
// only while TruncationSuffix is ASCII: spell it as the single-rune "…" and a budget of
// 1 or 2 emits a broken rune from inside the helper every caller trusts. Swept across
// every budget rather than at hand-picked ones, because the defect lives at exactly the
// budgets nobody writes a case for.
func TestTruncateUTF8BytesWithEllipsisNeverEmitsABrokenRuneAtAnyBudget(t *testing.T) {
	inputs := []string{
		"",
		"ascii only",
		"héllo wörld",
		"日本語のテキストです",
		"🙂🙃🙂 emoji run",
		strings.Repeat("é", 40),
	}

	var checked int
	for _, in := range inputs {
		for maxBytes := -2; maxBytes <= len(in)+4; maxBytes++ {
			got := TruncateUTF8BytesWithEllipsis(in, maxBytes)
			checked++
			if !utf8.ValidString(got) {
				t.Errorf("TruncateUTF8BytesWithEllipsis(%q, %d) = %q, which is not valid UTF-8", in, maxBytes, got)
			}
			if maxBytes > 0 && len(got) > maxBytes {
				t.Errorf("TruncateUTF8BytesWithEllipsis(%q, %d) returned %d bytes, over budget", in, maxBytes, len(got))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no budgets exercised; this sweep would pass vacuously")
	}

	// The floor for the branch above: it is only reachable while the suffix is short
	// enough for a budget to fall inside it.
	if len(TruncationSuffix) == 0 {
		t.Error("TruncationSuffix is empty, so the small-budget branch this sweep guards is unreachable")
	}
}
