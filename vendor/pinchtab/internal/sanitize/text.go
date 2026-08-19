package sanitize

import (
	"regexp"
	"strings"
	"unicode"
)

const TruncationSuffix = "..."

var (
	ansiCSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	unixAbs = regexp.MustCompile(`(^|[\s"'(=:])((?:/Users|/home|/var|/tmp|/private|/opt|/etc|/Volumes)(?:/[^\s"'():;<>{}\[\]]+)+)`)
	winAbs  = regexp.MustCompile(`(^|[\s"'(=:])([A-Za-z]:\\(?:[^\s"'():;<>{}\[\]]+\\?)+)`)
)

// TruncateUTF8BytesExact returns the longest prefix of s that fits in maxBytes bytes
// without splitting a rune. Nothing is appended: callers that want a
// truncation marker use TruncateUTF8BytesWithEllipsis.
func TruncateUTF8BytesExact(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	cut := 0
	for i := range s {
		if i > maxBytes {
			break
		}
		cut = i
	}
	return s[:cut]
}

func TruncateUTF8BytesWithEllipsis(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= len(TruncationSuffix) {
		return TruncationSuffix[:maxBytes]
	}
	return TruncateUTF8BytesExact(s, maxBytes-len(TruncationSuffix)) + TruncationSuffix
}

func StripANSI(s string) string {
	return ansiCSI.ReplaceAllString(s, "")
}

func StripControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	lastWasSpace := false
	for _, r := range s {
		if unicode.IsControl(r) {
			if !lastWasSpace {
				b.WriteByte(' ')
				lastWasSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastWasSpace = r == ' '
	}
	return strings.TrimSpace(b.String())
}

func RedactAbsolutePaths(s string) string {
	s = unixAbs.ReplaceAllString(s, `$1[path]`)
	s = winAbs.ReplaceAllString(s, `$1[path]`)
	return s
}

// StripTextSpoofingRunes removes the Unicode format characters that let text
// render as something other than its contents: bidirectional overrides and
// isolates (the Trojan Source class, where a right-to-left override makes
// "gnp.exe" display as "exe.png"), and zero-width blanks that hide word
// boundaries.
//
// StripControlChars cannot catch these — unicode.IsControl only covers category
// Cc, and these are Cf. The list is deliberate rather than "every Cf rune":
// zero-width joiner and non-joiner are required to render emoji sequences and
// Indic/Arabic script, so removing them would corrupt legitimate text.
func StripTextSpoofingRunes(s string) string {
	if !strings.ContainsFunc(s, isTextSpoofingRune) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isTextSpoofingRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isTextSpoofingRune(r rune) bool {
	switch r {
	case '\u200B', '\uFEFF', // zero-width space, byte order mark
		'\u200E', '\u200F', // left-to-right / right-to-left mark
		'\u202A', '\u202B', '\u202C', '\u202D', '\u202E', // embeddings and overrides
		'\u2066', '\u2067', '\u2068', '\u2069': // isolates
		return true
	}
	return false
}

func CleanForLog(s string, maxBytes int) string {
	return TruncateUTF8BytesWithEllipsis(StripTextSpoofingRunes(StripControlChars(StripANSI(s))), maxBytes)
}

func CleanError(s string, maxBytes int) string {
	return TruncateUTF8BytesWithEllipsis(RedactAbsolutePaths(StripTextSpoofingRunes(StripControlChars(StripANSI(s)))), maxBytes)
}
