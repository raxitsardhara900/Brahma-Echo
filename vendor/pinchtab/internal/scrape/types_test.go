package scrape

import (
	"os"
	"strings"
	"testing"
)

// The literal here is deliberately a second copy of SchemaVersion, and it is the
// point of the test. Every other assertion compares the report's field to the
// constant that stamped it, so it can prove the report is stamped but never that
// the version is the intended one — a bump passes the whole Go suite and surfaces
// only in the Docker e2e, long after it lands.
//
// This does not check correctness. It forces a bump to be acknowledged, together
// with the sibling artefacts that carry the same number.
func TestSchemaVersionIsPinnedSoABumpMustBeAcknowledged(t *testing.T) {
	const pinned = "2.0"

	if SchemaVersion != pinned {
		t.Fatalf("scrape SchemaVersion = %q, pinned at %q.\n"+
			"A scrape schema bump is a breaking change for report consumers. If it is intended, update all three:\n"+
			"  1. this literal,\n"+
			"  2. the schema history in docs/scrape.md — say which fields changed, as 1.0 -> 2.0 did for totalDiscovered -> totalURLsInSitemap,\n"+
			"  3. EXPECTED_SCRAPE_SCHEMA in tests/e2e/scenarios/cli/scrape-basic.sh.",
			SchemaVersion, pinned)
	}
}

// Item 2 of the checklist above is the only one nothing enforces: the pin itself
// carries item 1 and the Docker e2e carries item 3, so a bump documented nowhere
// passes every suite. This ties the docs to the constant rather than to the pinned
// literal, so it fires exactly when the history was not written.
func TestSchemaHistoryDocumentsTheCurrentVersion(t *testing.T) {
	const heading = "### Schema history"

	raw, err := os.ReadFile("../../docs/scrape.md")
	if err != nil {
		t.Fatal(err)
	}

	section := string(raw)
	start := strings.Index(section, heading)
	if start < 0 {
		t.Fatalf("docs/scrape.md has no %q section, so a schema bump has nowhere a report consumer would look", heading)
	}
	section = section[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}

	if !strings.Contains(section, SchemaVersion) {
		t.Errorf("the %q section of docs/scrape.md never mentions the current version %q — a consumer holding an older report has the version to compare against and nowhere to learn what changed",
			heading, SchemaVersion)
	}
}
