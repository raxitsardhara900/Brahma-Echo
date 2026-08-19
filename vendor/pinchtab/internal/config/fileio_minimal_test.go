package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A save must not materialise defaults. LoadFileConfig unmarshals the user's file on
// top of the shipped defaults, so the in-memory struct is always fully populated; a
// wholesale marshal turned a 50-byte config into a 3.8kB snapshot of one build's
// defaults, and every untouched setting became explicitly set — which silently cut that
// install off from every future default change.
func TestSavingAConfigDoesNotMaterialiseDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := "{\n  \"server\": {\n    \"port\": \"9913\",\n    \"token\": \"tok3\"\n  }\n}\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Errorf("a load-then-save round trip rewrote the file:\n before %q\n after  %q", original, string(after))
	}

	// Named individually rather than by size, so the failure says which section
	// leaked back in.
	for _, key := range []string{"$schema", "configVersion", "timeouts", "scheduler", "observability", "sessions", "autoSolver", "browsers", "profiles", "security", "instanceDefaults", "multiInstance"} {
		if strings.Contains(string(after), "\""+key+"\"") {
			t.Errorf("save added %q, which the user never set — that key now stops tracking the shipped default", key)
		}
	}
}

// The absent key must still resolve to the shipped value: suppressing the write is only
// safe because the loader treats absence as "use the default".
func TestAnAbsentSectionStillResolvesToTheShippedDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	shipped := DefaultFileConfig()
	if fc.Timeouts != shipped.Timeouts {
		t.Errorf("timeouts resolved to %+v, want the shipped %+v", fc.Timeouts, shipped.Timeouts)
	}
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(body), "timeouts") {
		t.Errorf("timeouts was written into a config that never set it: %s", body)
	}
}

// A key the file already carries is rewritten unconditionally, so an edit back to the
// shipped default still lands. A pure difference-from-defaults render would drop it from
// the patch and leave the old value standing — the one way writing less loses data.
func TestAnEditBackToTheDefaultValueIsStillWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"},"browser":{"version":"1.2.3.4"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	shipped := DefaultFileConfig()
	fc.Browser.BrowserVersion = shipped.Browser.BrowserVersion
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "1.2.3.4") {
		t.Errorf("the stale value survived a write that set it back to the default: %s", body)
	}
	if !strings.Contains(string(body), shipped.Browser.BrowserVersion) {
		t.Errorf("the key the user had set was dropped instead of updated: %s", body)
	}
}

// Member order is part of the file the user authored. Rendering through a map would sort
// every key, so a write the user asked for would arrive with an unexplained whole-file
// diff — the same harm as materialising defaults, wearing different clothes.
func TestSavingPreservesTheFilesKeyOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := "{\n  \"server\": {\n    \"token\": \"tok3\",\n    \"port\": \"9913\",\n    \"bind\": \"127.0.0.1\"\n  },\n  \"browser\": {\n    \"version\": \"1.2.3.4\"\n  }\n}\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != original {
		t.Errorf("save reordered or reformatted an unchanged file:\n before %q\n after  %q", original, string(body))
	}
	if got := strings.Index(string(body), "token"); got > strings.Index(string(body), "port") {
		t.Error("server.token moved after server.port; the file's own order was not preserved")
	}
}

// A key the struct does not model must survive a save. Dropping it would delete
// something the writer merely failed to parse.
func TestSavingKeepsAKeyTheStructDoesNotModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"},"somethingNobodyModels":{"keep":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "somethingNobodyModels") {
		t.Errorf("an unmodelled key was deleted by a save: %s", body)
	}
}

// The e2e fixture is the realistic case: a hand-authored, richly-populated config that a
// developer points PINCHTAB_CONFIG at when reproducing a failure. A load-then-save must
// leave it byte-identical, including its inline arrays.
func TestSavingTheE2EFixtureLeavesItByteIdentical(t *testing.T) {
	source := filepath.Join("..", "..", "tests", "e2e", "config", "pinchtab.json")
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Errorf("a load-then-save changed the e2e fixture; first divergence at byte %d", firstDiff(string(original), string(after)))
	}
	for _, forbidden := range []string{"$schema", "configVersion", userConfigDir()} {
		if forbidden != "" && strings.Contains(string(after), forbidden) {
			t.Errorf("save added %q to the fixture", forbidden)
		}
	}
}

func firstDiff(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// No absolute host path may reach a config the user did not put one in: such a file gets
// copied into containers, committed, and shared.
func TestSavingWritesNoAbsoluteHostPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	fc.ConfigVersion = CurrentConfigVersion
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"baseDir", "extensionPaths"} {
		if strings.Contains(string(body), key) {
			t.Errorf("save wrote %q into a config that never set it: %s", key, body)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.Contains(string(body), home) {
		t.Errorf("save baked a host home directory into the config: %s", body)
	}
	// The write the wizard needs still lands.
	var written map[string]any
	if err := json.Unmarshal(body, &written); err != nil {
		t.Fatal(err)
	}
	if written["configVersion"] != CurrentConfigVersion {
		t.Errorf("configVersion = %v, want %q — the one key the startup write exists to add", written["configVersion"], CurrentConfigVersion)
	}
}

// Loading must not unprotect a config the user marked read-only: restoring the owner
// write bit on the read path is what let a later write replace it silently.
func TestLoadingLeavesAReadOnlyConfigsModeAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only file mode semantics")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0444 {
		t.Errorf("mode after load = %o, want 0444 — reading a config must not make it writable again", got)
	}

	// And the write that used to succeed because of that chmod now fails loudly
	// instead of replacing a protected file. The change has to be real: a save with
	// nothing to say is skipped entirely, which is its own form of leaving the file
	// alone.
	fc.ConfigVersion = CurrentConfigVersion
	if err := SaveFileConfig(fc, path); err == nil {
		t.Error("SaveFileConfig succeeded against a 0444 config; a protected file must not be replaced silently")
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := fi.Mode().Perm(); got != 0444 {
		t.Errorf("mode after a refused save = %o, want 0444", got)
	}
}

// security.idpi.allowedDomains is the one alias the loader normalises into
// security.allowedDomains. A patch-based save writes the canonical key while the alias
// stays on disk, so the round trip is what matters: the value the user set must survive,
// and the loader must keep preferring the canonical spelling.
func TestSavingAConfigThatUsesTheIDPIAllowedDomainsAliasKeepsTheValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"port":"9913","token":"tok3"},"security":{"idpi":{"allowedDomains":["example.test"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)

	fc, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(fc.Security.AllowedDomains, ","); got != "example.test" {
		t.Fatalf("precondition: the alias must load into Security.AllowedDomains, got %q", got)
	}
	if err := SaveFileConfig(fc, path); err != nil {
		t.Fatal(err)
	}

	reloaded, _, err := LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(reloaded.Security.AllowedDomains, ","); got != "example.test" {
		body, _ := os.ReadFile(path)
		t.Errorf("the aliased value did not survive a save: got %q, file is now %s", got, body)
	}
}
