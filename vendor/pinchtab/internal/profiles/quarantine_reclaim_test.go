package profiles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/srccensus"
)

type pruneResult struct {
	Removed    bool  `json:"removed"`
	Count      int   `json:"count"`
	TotalBytes int64 `json:"totalBytes"`
	Profiles   []struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		Bytes int64  `json:"bytes"`
	} `json:"profiles"`
}

func writeSizedProfile(t *testing.T, baseDir, name string, bytes int) {
	t.Helper()

	dir := filepath.Join(baseDir, name, "Default")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "State"), make([]byte, bytes), 0o600); err != nil {
		t.Fatal(err)
	}
}

func reclaimBase(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	writeSizedProfile(t, base, "default", 100)
	writeSizedProfile(t, base, "default.quarantine-1700000001", 200)
	writeSizedProfile(t, base, "work.quarantine-1700000002", 300)
	return base
}

func postPrune(t *testing.T, pm *ProfileManager, target string, body string) (int, pruneResult) {
	t.Helper()

	mux := http.NewServeMux()
	pm.RegisterHandlers(mux)

	req := httptest.NewRequest(http.MethodPost, target, http.NoBody)
	if body != "" {
		req = httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var out pruneResult
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("prune response is not the documented shape (%v): %s", err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func quarantineDirsOnDisk(t *testing.T, base string) []string {
	t.Helper()

	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && bridge.IsQuarantinedProfileDir(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names
}

// The bare invocation reports and deletes nothing. The assertion that matters is the
// negative one — the directories are still on disk with their contents — because a
// handler that deleted while reporting the same total would pass a total-only check.
func TestBarePruneReportsTheBacklogAndDeletesNothing(t *testing.T) {
	base := reclaimBase(t)
	pm := NewProfileManager(base)

	code, out := postPrune(t, pm, "/profiles/prune", "")

	if code != http.StatusOK {
		t.Fatalf("POST /profiles/prune = %d, want 200", code)
	}
	if out.Removed {
		t.Error("the bare invocation reported removed=true")
	}
	if out.Count != 2 {
		t.Fatalf("count = %d, want the two quarantined profiles: %+v", out.Count, out.Profiles)
	}
	if out.TotalBytes != 500 {
		t.Errorf("totalBytes = %d, want 500", out.TotalBytes)
	}

	if got := quarantineDirsOnDisk(t, base); len(got) != 2 {
		t.Fatalf("quarantined directories after a bare prune = %v, want both still there", got)
	}
	for _, prof := range out.Profiles {
		if _, err := os.Stat(filepath.Join(prof.Path, "Default", "State")); err != nil {
			t.Errorf("%s lost its contents to a bare prune: %v", prof.Name, err)
		}
	}
}

// Real directories on disk, not a mock: the reclaimed total is the bytes the files
// actually held, and the tree afterwards holds only the live profile.
func TestConfirmedPruneRemovesTheQuarantinedDirectoriesAndReportsWhatItFreed(t *testing.T) {
	base := reclaimBase(t)
	pm := NewProfileManager(base)

	code, out := postPrune(t, pm, "/profiles/prune", `{"confirm":true}`)

	if code != http.StatusOK {
		t.Fatalf("POST /profiles/prune = %d, want 200", code)
	}
	if !out.Removed || out.Count != 2 {
		t.Fatalf("removed = %v count = %d, want both quarantines removed", out.Removed, out.Count)
	}
	if out.TotalBytes != 500 {
		t.Errorf("totalBytes = %d, want 500", out.TotalBytes)
	}
	if got := quarantineDirsOnDisk(t, base); len(got) != 0 {
		t.Fatalf("quarantined directories after a confirmed prune = %v, want none", got)
	}
	if _, err := os.Stat(filepath.Join(base, "default", "Default", "State")); err != nil {
		t.Errorf("the live profile was removed by a prune: %v", err)
	}
}

func TestConfirmMayAlsoArriveOnTheQueryString(t *testing.T) {
	base := reclaimBase(t)
	pm := NewProfileManager(base)

	code, out := postPrune(t, pm, "/profiles/prune?confirm=true", "")

	if code != http.StatusOK || !out.Removed {
		t.Fatalf("POST /profiles/prune?confirm=true = %d removed=%v, want a confirmed removal", code, out.Removed)
	}
	if got := quarantineDirsOnDisk(t, base); len(got) != 0 {
		t.Errorf("quarantined directories = %v, want none", got)
	}
}

// A live profile is unreachable however the request is shaped. Naming one is refused
// because the selector is matched against directories that already passed the quarantine
// predicate, so a live name simply is not in the set.
func TestPruneRefusesToNameALiveProfile(t *testing.T) {
	base := reclaimBase(t)
	pm := NewProfileManager(base)

	code, _ := postPrune(t, pm, "/profiles/prune", `{"confirm":true,"profile":"default"}`)

	if code != http.StatusBadRequest {
		t.Fatalf("naming a live profile = %d, want 400", code)
	}
	if _, err := os.Stat(filepath.Join(base, "default", "Default", "State")); err != nil {
		t.Fatalf("the live profile named in a refused prune was removed: %v", err)
	}
	if got := quarantineDirsOnDisk(t, base); len(got) != 2 {
		t.Errorf("a refused prune still changed the tree: %v", got)
	}
}

// The caller names a profile or asks for all of them, never a filesystem path.
func TestPruneRefusesAPathOutsideTheProfileRoot(t *testing.T) {
	outsideRoot := t.TempDir()
	writeSizedProfile(t, outsideRoot, "victim.quarantine-1700000009", 64)

	for _, name := range []string{
		"../victim.quarantine-1700000009",
		"../../etc",
		filepath.Join(outsideRoot, "victim.quarantine-1700000009"),
	} {
		t.Run(name, func(t *testing.T) {
			base := reclaimBase(t)
			pm := NewProfileManager(base)

			body, err := json.Marshal(map[string]any{"confirm": true, "profile": name})
			if err != nil {
				t.Fatal(err)
			}
			code, _ := postPrune(t, pm, "/profiles/prune", string(body))

			if code != http.StatusBadRequest {
				t.Fatalf("prune of %q = %d, want 400", name, code)
			}
			if got := quarantineDirsOnDisk(t, base); len(got) != 2 {
				t.Errorf("a refused prune still changed the tree: %v", got)
			}
		})
	}

	if _, err := os.Stat(filepath.Join(outsideRoot, "victim.quarantine-1700000009", "Default", "State")); err != nil {
		t.Fatalf("a directory outside the profiles base was removed: %v", err)
	}
}

// A profile whose own name ends in the quarantine suffix IS reclaimed — it is
// byte-identical on disk to a real quarantine and nothing can tell them apart. What
// bounds it: the shape is one the product reserves and writes itself, the listing already
// shows such a directory as quarantined so the user sees it before confirming, and the
// removal only happens on an explicit confirm.
func TestPruneTakesALookalikeProfileAndTheListingWarnedAboutIt(t *testing.T) {
	base := t.TempDir()
	writeSizedProfile(t, base, "default", 100)
	writeSizedProfile(t, base, "mine.quarantine-1700000009", 64)
	pm := NewProfileManager(base)

	listed := findListed(listProfiles(t, pm, ""), "mine.quarantine-1700000009")
	if listed == nil {
		t.Fatal("the lookalike is absent from the listing, so a user could not see it before confirming")
	}
	if listed["quarantined"] != true {
		t.Errorf("the lookalike lists as quarantined = %v, want true — the warning a user gets is the listing", listed["quarantined"])
	}

	code, out := postPrune(t, pm, "/profiles/prune", `{"confirm":true}`)

	if code != http.StatusOK || out.Count != 1 {
		t.Fatalf("prune = %d count = %d, want the lookalike removed", code, out.Count)
	}
	if _, err := os.Stat(filepath.Join(base, "default", "Default", "State")); err != nil {
		t.Errorf("the live profile beside the lookalike was removed: %v", err)
	}
}

// After a reclaim the listing and the health count must agree with the disk. The health
// envelope counts p.Quarantined over profileLister.List(), an interface with List as its
// ONLY method, so it cannot read anything but what is asserted here.
func TestTheListingAndTheHealthCountAgreeWithTheDiskAfterAReclaim(t *testing.T) {
	base := reclaimBase(t)
	pm := NewProfileManager(base)

	if code, _ := postPrune(t, pm, "/profiles/prune", `{"confirm":true}`); code != http.StatusOK {
		t.Fatalf("prune = %d, want 200", code)
	}

	onDisk := len(quarantineDirsOnDisk(t, base))

	listed := listProfiles(t, pm, "?all=true")
	listedQuarantined := 0
	for _, entry := range listed {
		if entry["quarantined"] == true {
			listedQuarantined++
		}
	}

	infos, err := pm.List()
	if err != nil {
		t.Fatal(err)
	}
	healthQuarantined := 0
	for _, info := range infos {
		if info.Quarantined {
			healthQuarantined++
		}
	}

	if onDisk != 0 || listedQuarantined != onDisk || healthQuarantined != onDisk {
		t.Errorf("after a reclaim disk = %d, listing = %d, health = %d; all three must agree", onDisk, listedQuarantined, healthQuarantined)
	}
	if len(listed) != 1 {
		t.Errorf("listing has %d entries, want just the live profile: %v", len(listed), listed)
	}
}

// Exposing ProfileManager.Delete as the reclaim route is the short path the card forbids:
// it resolves a name through the profile listing and removes what it finds with no
// eligibility rule, so a reclaim routed through it becomes a second, looser way to reach
// a live profile. A prose prohibition is one the next contributor takes anyway, so it is
// asserted here — implement it and this reddens by name.
func TestReclaimNeverReachesTheUnguardedProfileDeleter(t *testing.T) {
	pkg := srccensus.Load(t, ".", 5)

	fn, ok := pkg.Func("handlePruneQuarantined")
	if !ok {
		t.Fatal("handlePruneQuarantined not found; the reclaim route was renamed and this census is reading nothing")
	}

	// Both spellings of each: the AST callee for a method call carries the receiver name,
	// so banning the bare identifier alone would match a free function and miss pm.Delete
	// — which is the call this census exists to catch.
	banned := []string{
		"pm.Delete", "Delete",
		"pm.ForceDelete", "ForceDelete",
		"pm.remove", "remove",
		"pm.findProfileDirByName", "findProfileDirByName",
		"pm.resolveIDOnly", "resolveIDOnly",
		"os.RemoveAll",
	}
	for _, name := range banned {
		for _, site := range pkg.CallsAllowingNone(name) {
			if pkg.Contains(fn, site) {
				t.Errorf("the reclaim route calls %s at %s — that reaches a directory with no quarantine predicate, which is exactly the second, looser deleter this route must not become", name, site)
			}
		}
	}

	// The floor under the receiver-qualified half: pm.Delete must still be the spelling
	// this package uses, or the ban above is checking a name nothing is called by. Calls
	// fails when it matches nothing, so renaming the receiver reddens here instead of
	// quietly disarming the census.
	pkg.Calls(t, "pm.Delete")

	reached := false
	for _, site := range pkg.CallsAllowingNone("bridge.ReclaimQuarantinedProfiles") {
		if pkg.Contains(fn, site) {
			reached = true
		}
	}
	if !reached {
		t.Error("the reclaim route no longer calls bridge.ReclaimQuarantinedProfiles, so it removes quarantined profiles some other way and the predicate is not shared")
	}
}
