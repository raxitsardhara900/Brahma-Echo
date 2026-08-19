package remedy

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/srccensus"
)

// moduleRoot is this package's distance to the module root; the census is module-wide
// because the rule is about every producer, wherever it lands.
const moduleRoot = "../.."

// The floors are vacuity checks. They are counts of what the walk must still be finding, not
// a list of producers: the producers are whatever the walk returns.
const (
	minModuleFiles   = 300
	minDeclareSites  = 8
	ownerPackagePath = "internal/remedy/"
)

// sitesContaining returns every non-test line in the module holding needle. skipOwner drops
// this package, whose job is to hold the shapes the rule forbids elsewhere.
func sitesContaining(t *testing.T, needle string, skipOwner bool) []string {
	t.Helper()

	var sites []string
	for _, file := range srccensus.Tree(t, moduleRoot, minModuleFiles) {
		if skipOwner && strings.HasPrefix(file.Name, ownerPackagePath) {
			continue
		}
		for i, line := range strings.Split(file.Text, "\n") {
			if strings.Contains(line, needle) {
				sites = append(sites, fmt.Sprintf("%s:%d: %s", file.Name, i+1, strings.TrimSpace(line)))
			}
		}
	}
	return sites
}

// A producer that writes the field by hand is a producer outside the contract, and that is
// exactly how the field came to mean four things: eight sites each wrote their own value into
// their own details map. The key may be built in ONE place, so the property is checked where
// the value is written.
//
// If a legitimate site needs more fields than Details returns, add to the map Details returns
// — do not re-point this guard at a second writer.
func TestNoSiteOutsideThisPackageWritesTheRemedyField(t *testing.T) {
	if offenders := sitesContaining(t, `"remedy":`, true); len(offenders) > 0 {
		t.Errorf("these sites write details.remedy by hand instead of through remedy.Details, so nothing checks what they publish:\n%s",
			strings.Join(offenders, "\n"))
	}
}

// The vacuity floor for the guard above, and the census the card asked for: zero matches is
// its PASS condition, so the walk has to be shown to be finding the producers somewhere.
// Every remedy in the repo is declared, and the command-tree guard in cmd/pinchtab walks
// exactly what Declare registers.
func TestEveryRemedyInTheRepoIsDeclaredHere(t *testing.T) {
	sites := sitesContaining(t, "remedy.Declare(", true)
	if len(sites) < minDeclareSites {
		t.Fatalf("found %d remedy.Declare sites, want at least %d; the producers have stopped going through the constructor, so re-point this guard at whatever replaced it rather than lowering the floor:\n%s",
			len(sites), minDeclareSites, strings.Join(sites, "\n"))
	}
}

// The RENDERED slot is what a caller actually reads, and it cannot be told apart from a
// details.remedy by looking. A second site printing prose into "Remedy:" defeats the contract
// however honest the struct it came from — the CLI's own stale-tab advice used to do exactly
// that. So the slot has one writer, and this guard is keyed on the slot rather than on the
// field name for that reason.
func TestTheRenderedRemedySlotHasOneWriter(t *testing.T) {
	sites := sitesContaining(t, "Remedy: ", false)
	if len(sites) != 1 {
		t.Errorf("the \"Remedy:\" slot is written at %d sites, want exactly 1 — a second writer publishes prose into the line a caller reads as executable:\n%s",
			len(sites), strings.Join(sites, "\n"))
	}
}
