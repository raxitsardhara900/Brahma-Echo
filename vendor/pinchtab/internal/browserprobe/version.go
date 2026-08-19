package browserprobe

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// FallbackChromeVersion is the Chrome version advertised when the real one is not
// known — no browser.version in config and no successful probe of the launched binary.
//
// It lives here, beside RunVersion, because this package is the only leaf all three
// consumers can import: internal/stealth imports internal/config, so config can never
// read a constant owned by stealth, and internal/handlers reads both. Before it had one
// owner the same digits were written out in config's file default, config's runtime
// default and two fallbacks in stealth's UA builder, so a Chrome release meant four
// edits in lockstep across three packages and nothing failed when they drifted.
//
// Bumping it is a stopgap, not the fix: the version a page sees should come from the
// binary RunVersion probes, and this is what is left when that cannot be answered.
const FallbackChromeVersion = "144.0.7559.133"

func RunVersion(ctx context.Context, binary string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, binary, "--version").CombinedOutput()
	if err != nil {
		return "", err
	}
	line := parseFirstLine(string(out))
	if line == "" {
		line = strings.TrimSpace(string(out))
	}
	return line, nil
}

func parseFirstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

func ExtractVersionToken(s string) string {
	fields := strings.Fields(s)
	for _, f := range fields {
		f = strings.Trim(f, "()[],;")
		if hasDottedDigits(f) {
			return f
		}
	}
	return ""
}

func hasDottedDigits(s string) bool {
	sawDigit := false
	sawDot := false
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			sawDigit = true
		case c == '.':
			sawDot = true
		default:
			return false
		}
	}
	return sawDigit && sawDot
}

// CompareSemver returns -1/0/1 comparing major.minor.patch of a and b; it is
// lenient about "v" prefixes, build/prerelease suffixes, and non-numeric noise.
func CompareSemver(a, b string) int {
	ap := splitVersion(a)
	bp := splitVersion(b)
	for i := 0; i < 3; i++ {
		switch {
		case ap[i] < bp[i]:
			return -1
		case ap[i] > bp[i]:
			return 1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := [3]int{}
	segs := strings.SplitN(v, ".", 3)
	for i, s := range segs {
		if i >= 3 {
			break
		}
		n := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
				continue
			}
			break
		}
		parts[i] = n
	}
	return parts
}
