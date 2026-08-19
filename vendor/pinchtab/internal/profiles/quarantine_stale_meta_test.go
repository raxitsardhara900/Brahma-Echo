package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

// quarantineCarryingStaleMeta builds the on-disk state quarantine leaves when the
// profile.json removal does not succeed: a quarantined directory still holding metadata
// that names the live profile. Quarantine renames the directory and only WARNS if it
// cannot drop the file afterwards, and it deliberately proceeds while a dying browser may
// still hold the directory, so this state is reachable rather than hypothetical.
func quarantineCarryingStaleMeta(t *testing.T, baseDir, name string) (liveDir, quarantineDir string) {
	t.Helper()

	id := profileID(name)
	liveDir = writeListableProfile(t, baseDir, id)
	if err := writeProfileMeta(liveDir, ProfileMeta{ID: id, Name: name}); err != nil {
		t.Fatal(err)
	}

	quarantineDir = writeListableProfile(t, baseDir, id+".quarantine-1700000001")
	if err := writeProfileMeta(quarantineDir, ProfileMeta{ID: id, Name: name}); err != nil {
		t.Fatal(err)
	}
	return liveDir, quarantineDir
}

// The invariant trustedProfileMeta exists for: a quarantined directory must never answer
// to the live profile's identity. Its profile.json still says otherwise, and ProfileID
// hashes the name, so trusting that file would give two directories one name AND one ID —
// after which a lookup, a listing row or a DELETE by id can land on either. Nothing here
// tests the removal that normally prevents the state; the point is that the identity rules
// hold even when it has not happened.
func TestAQuarantineCarryingStaleMetadataNeverClaimsTheLiveProfileIdentity(t *testing.T) {
	baseDir := t.TempDir()
	liveDir, quarantineDir := quarantineCarryingStaleMeta(t, baseDir, "shopping")
	pm := NewProfileManager(baseDir)

	// This one is defended by two accidents, and neither is the rule under test. The live
	// directory is reachable by its own name, so the direct stat in findProfileDirByName
	// returns before ReadDir is consulted at all; and even without it, a quarantine sorts
	// after the directory it was renamed from, so the id match wins first. Measured: either
	// accident alone keeps this subtest green, and so does weakening trustedProfileMeta.
	// The identity subtest below is what holds the line against that weakening, so do not
	// read a green here as covering the rule. Delete inherits this lookup's answer and is
	// green under the same weakening — but it is not inert: it reds once the by-name lookup
	// stops preferring the live directory, which is the change reclaiming quarantined
	// directories could plausibly make.
	t.Run("a lookup by name resolves the live directory", func(t *testing.T) {
		got, err := pm.findProfileDirByName("shopping")
		if err != nil {
			t.Fatalf("findProfileDirByName() error = %v", err)
		}
		if got != liveDir {
			t.Errorf("resolved %q, want the live directory %q — resolving the quarantine would point every later write at evidence", got, liveDir)
		}
	})

	t.Run("the two directories keep distinct names and ids", func(t *testing.T) {
		listed, err := pm.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(listed) != 2 {
			t.Fatalf("listed %d profiles, want the live one and its quarantine", len(listed))
		}

		names := map[string]int{}
		ids := map[string]int{}
		for _, profile := range listed {
			names[profile.Name]++
			ids[profile.ID]++
		}
		for name, count := range names {
			if count > 1 {
				t.Errorf("%d listed profiles share the name %q; the quarantine is claiming the live profile's identity", count, name)
			}
		}
		for id, count := range ids {
			if count > 1 {
				t.Errorf("%d listed profiles share the id %q, so a DELETE by id can reach either directory", count, id)
			}
		}
	})

	t.Run("deleting the profile by name leaves the quarantine on disk", func(t *testing.T) {
		if err := pm.Delete("shopping"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := os.Stat(liveDir); !os.IsNotExist(err) {
			t.Errorf("live directory still present after Delete: %v", err)
		}
		if _, err := os.Stat(quarantineDir); err != nil {
			t.Errorf("Delete removed the quarantined directory instead of, or as well as, the profile: %v", err)
		}
		if _, err := os.Stat(filepath.Join(quarantineDir, "Default")); err != nil {
			t.Errorf("the quarantined directory lost its contents: %v", err)
		}
	})
}
