package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
)

func writeListableProfile(t *testing.T, baseDir, name string) string {
	t.Helper()

	path := filepath.Join(baseDir, name)
	if err := os.MkdirAll(filepath.Join(path, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func countQuarantined(t *testing.T, pm *ProfileManager) (live, quarantined int) {
	t.Helper()

	listed, err := pm.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, profile := range listed {
		if profile.Quarantined {
			quarantined++
			continue
		}
		live++
	}
	return live, quarantined
}

// The listing and the /health counters derived from it read the directory each time and
// flag entries through the same predicate the prune goes through, so a prune is visible
// immediately and no count can outlive the disk. This drives both halves together, which
// is the only way to see a cache if one were ever introduced.
func TestTheProfileListingFollowsDiskAfterAQuarantinePrune(t *testing.T) {
	baseDir := t.TempDir()
	profileDir := writeListableProfile(t, baseDir, "default")
	writeListableProfile(t, baseDir, "default.quarantine-1700000001")
	writeListableProfile(t, baseDir, "default.quarantine-1700000002")
	newest := writeListableProfile(t, baseDir, "default.quarantine-1700000003")

	pm := NewProfileManager(baseDir)
	live, quarantined := countQuarantined(t, pm)
	if live != 1 || quarantined != 3 {
		t.Fatalf("before the prune: live=%d quarantined=%d, want 1 and 3", live, quarantined)
	}

	removals, err := bridge.PruneQuarantinedProfiles(profileDir, newest, 1)
	if err != nil {
		t.Fatalf("PruneQuarantinedProfiles() error = %v", err)
	}
	if len(removals) != 2 {
		t.Fatalf("removals = %v, want the two older quarantines", removals)
	}

	live, quarantined = countQuarantined(t, pm)
	if live != 1 || quarantined != 1 {
		t.Errorf("after the prune: live=%d quarantined=%d, want 1 and 1 — the listing must agree with the disk", live, quarantined)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != live+quarantined {
		t.Errorf("the listing reports %d profiles but %d directories remain on disk", live+quarantined, len(entries))
	}
}
