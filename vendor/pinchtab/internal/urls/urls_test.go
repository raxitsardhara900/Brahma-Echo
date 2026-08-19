package urls

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pinchtab/pinchtab/internal/sanitize"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// No protocol - should add https://
		{"example.com", "https://example.com"},
		{"example.com/path", "https://example.com/path"},
		{"example.com:8080", "https://example.com:8080"},
		{"sub.example.com/path?q=1", "https://sub.example.com/path?q=1"},

		// Already has protocol - should not modify
		{"https://example.com", "https://example.com"},
		{"http://example.com", "http://example.com"},
		{"http://localhost:8080", "http://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Normalize(tt.input)
			if result != tt.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		// Valid URLs
		{"https://example.com", false},
		{"http://example.com", false},
		{"https://example.com/path", false},
		{"http://localhost:8080", false},
		{"example.com", false},          // normalized to https://
		{"example.com/path?q=1", false}, // normalized to https://

		// Invalid URLs
		{"", true}, // empty is the only invalid case
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Sanitize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Sanitize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	valid := []string{
		"https://example.com",
		"example.com",
		"file:///path/to/file.html",
		"javascript:alert(1)",
		"chrome://settings",
	}
	for _, u := range valid {
		if !IsValid(u) {
			t.Errorf("expected %q to be valid", u)
		}
	}
	if IsValid("") {
		t.Error("expected empty string to be invalid")
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com", "example.com"},
		{"https://Example.COM/path", "example.com"},
		{"http://sub.example.com:8080/path", "sub.example.com"},
		{"example.com/path", "example.com"},
		{"EXAMPLE.COM", "example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractHost(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractHost(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitize_BrowserURLs(t *testing.T) {
	// All explicit schemes should be allowed — user knows what they're doing
	validURLs := []string{
		"http://example.com",
		"https://example.com",
		"file:///path/to/file.html",
		"chrome://settings",
		"chrome://extensions",
		"chrome-extension://abc123/popup.html",
		"about:blank",
		"data:text/html,<h1>hi</h1>",
		"javascript:alert(1)",
		"javascript:void(0)",
		"vbscript:msgbox(1)",
		"ftp://files.example.com",
		"view-source:https://example.com",
	}
	for _, u := range validURLs {
		result, err := Sanitize(u)
		if err != nil {
			t.Errorf("expected %q to be valid, got error: %v", u, err)
		}
		if result != u {
			t.Errorf("expected %q unchanged, got %q", u, result)
		}
	}
}

func TestRedactForLog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://user:pass@Example.COM:8443/callback?code=secret#done", "https://example.com:8443/callback"},
		{"example.com/path?q=1", "https://example.com/path"},
		{"about:blank#frag", "about:blank"},
		{"", ""},
		{"://bad-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := RedactForLog(tt.input); got != tt.expected {
				t.Fatalf("RedactForLog(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// The byte cap on a redacted log URL moved from a package-private copy to
// sanitize.TruncateUTF8BytesWithEllipsis. These pin the empty, under-limit, at-limit and
// over-limit cases at a converted call site.
//
// Rune-boundary splitting is deliberately NOT pinned here and cannot be:
// RedactForLog normalizes through net/url first, which percent-encodes every
// non-ASCII byte, so the string reaching the truncator is always pure ASCII and
// no input can make the cut land inside a multi-byte rune. That case is pinned
// at the console-log call site, which caps raw text.
func TestRedactForLogTruncationBoundaries(t *testing.T) {
	host := "https://example.com/"
	fill := maxLogURLBytes - len(host)

	tests := []struct {
		name string
		raw  string
		want func(got string) error
	}{
		{
			name: "empty input stays empty",
			raw:  "",
			want: func(got string) error {
				if got != "" {
					return fmt.Errorf("got %q, want empty", got)
				}
				return nil
			},
		},
		{
			name: "under the limit is returned verbatim",
			raw:  host + strings.Repeat("a", 10),
			want: func(got string) error {
				if got != host+strings.Repeat("a", 10) {
					return fmt.Errorf("got %q, want the input unchanged", got)
				}
				return nil
			},
		},
		{
			name: "exactly at the limit is returned verbatim",
			raw:  host + strings.Repeat("a", fill),
			want: func(got string) error {
				if len(got) != maxLogURLBytes {
					return fmt.Errorf("len = %d, want %d", len(got), maxLogURLBytes)
				}
				if strings.HasSuffix(got, sanitize.TruncationSuffix) {
					return fmt.Errorf("a URL exactly at the limit must not be marked truncated: %q", got)
				}
				return nil
			},
		},
		{
			name: "over the limit is cut to the cap and marked",
			raw:  host + strings.Repeat("a", fill) + "€€€",
			want: func(got string) error {
				if !utf8.ValidString(got) {
					return fmt.Errorf("truncated URL is not valid UTF-8: %q", got)
				}
				if len(got) > maxLogURLBytes {
					return fmt.Errorf("len = %d, want <= %d", len(got), maxLogURLBytes)
				}
				if !strings.HasSuffix(got, sanitize.TruncationSuffix) {
					return fmt.Errorf("a cut URL must carry the truncation marker: %q", got)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.want(RedactForLog(tt.raw)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
