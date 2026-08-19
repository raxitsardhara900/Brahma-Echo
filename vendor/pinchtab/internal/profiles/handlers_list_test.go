package profiles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
)

func writeListedProfile(t *testing.T, baseDir, name string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(baseDir, name, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func listProfiles(t *testing.T, pm *ProfileManager, query string) []map[string]any {
	t.Helper()

	mux := http.NewServeMux()
	pm.RegisterHandlers(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles"+query, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /profiles%s = %d, want 200: %s", query, rec.Code, rec.Body.String())
	}

	var listed []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("response is not a JSON array (%v): %s", err, rec.Body.String())
	}
	return listed
}

func findListed(listed []map[string]any, name string) map[string]any {
	for _, entry := range listed {
		if entry["name"] == name {
			return entry
		}
	}
	return nil
}

// The classifications the API computes have to reach the wire, or a client reading them
// gets undefined for ever. quarantined already did; temporary did not — the response type
// had no such field, and the only listing that can contain a temporary profile served a
// different struct entirely.
func TestProfileListingServesBothClassificationFlags(t *testing.T) {
	baseDir := t.TempDir()
	writeListedProfile(t, baseDir, "default")
	writeListedProfile(t, baseDir, "instance-9868")
	writeListedProfile(t, baseDir, "default.quarantine-1700000001")
	pm := NewProfileManager(baseDir)

	all := listProfiles(t, pm, "?all=true")

	temporary := findListed(all, "instance-9868")
	if temporary == nil {
		t.Fatalf("all=true listing omitted the instance profile: %v", all)
	}
	if temporary["temporary"] != true {
		t.Errorf("instance profile temporary = %v, want true", temporary["temporary"])
	}
	if _, ok := temporary["sizeMB"]; !ok {
		t.Errorf("instance profile is served in a different shape from the rest: %v", temporary)
	}

	quarantined := findListed(all, "default.quarantine-1700000001")
	if quarantined == nil {
		t.Fatalf("all=true listing omitted the quarantined profile: %v", all)
	}
	if quarantined["quarantined"] != true {
		t.Errorf("quarantined profile quarantined = %v, want true", quarantined["quarantined"])
	}
	if !bridge.IsQuarantinedProfileDir("default.quarantine-1700000001") {
		t.Error("the fixture name is not one the quarantine predicate recognises, so this row proves nothing")
	}

	user := findListed(all, "default")
	if user == nil {
		t.Fatalf("all=true listing omitted the user profile: %v", all)
	}
	if user["temporary"] != false || user["quarantined"] != false {
		t.Errorf("user profile flags = temporary %v / quarantined %v, want both false", user["temporary"], user["quarantined"])
	}
}

// The default listing still hides temporary profiles, which is why the flag is dormant for
// the dashboard until a caller asks for all=true. Stated as a test so the filter is a
// recorded decision rather than a surprise to the next reader of the UI code.
func TestDefaultProfileListingStillHidesTemporaryProfilesButKeepsQuarantined(t *testing.T) {
	baseDir := t.TempDir()
	writeListedProfile(t, baseDir, "default")
	writeListedProfile(t, baseDir, "instance-9868")
	writeListedProfile(t, baseDir, "default.quarantine-1700000001")
	pm := NewProfileManager(baseDir)

	listed := listProfiles(t, pm, "")

	if findListed(listed, "instance-9868") != nil {
		t.Errorf("the default listing now includes a temporary profile: %v", listed)
	}
	if findListed(listed, "default.quarantine-1700000001") == nil {
		t.Errorf("the default listing dropped the quarantined profile, which an operator needs to see: %v", listed)
	}
	for _, entry := range listed {
		if _, ok := entry["temporary"]; !ok {
			t.Errorf("%v is served without the temporary field; the shape must not depend on the query", entry)
		}
	}
}
