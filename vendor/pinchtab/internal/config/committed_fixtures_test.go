package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/srccensus"
)

// Every config fixture the repo ships must load clean. The unknown-key walk's first find was
// the repo's own fixtures — six of them set keys declared nowhere, so every e2e and benchmark
// run logged a WARN about a batching knob that was never applied. Permanent noise is how a
// real warning later gets skimmed past.
//
// The population is DISCOVERED, not listed. A hand-written list encodes today's fixtures and
// is the same failure this guard cleans up: the next one added lands unchecked. The floor
// below is what keeps a walk that stopped matching from passing over nothing.
func TestEveryCommittedConfigFixtureLoadsWithNoUnknownKeys(t *testing.T) {
	fixtures := committedConfigFixtures(t)
	if len(fixtures) < committedConfigFixtureFloor {
		t.Fatalf("found %d config fixtures, want at least %d; the walk stopped matching and would pass vacuously — re-point it at wherever the fixtures moved rather than deleting it", len(fixtures), committedConfigFixtureFloor)
	}

	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path) // #nosec G304 -- path produced by this test's own walk.
			if err != nil {
				t.Fatal(err)
			}
			if keys := UnknownFileConfigKeys(raw); len(keys) > 0 {
				t.Errorf("%s sets %s; no config section declares them, so they do nothing and every run of this fixture logs a WARN — remove them, or add the setting to FileConfig and internal/schema/config.json if it was meant to be real", path, strings.Join(keys, ", "))
			}
		})
	}
}

// Well under the twelve that exist, so adding or retiring a fixture does not trip it while a
// walk that lost the tree still fails.
const committedConfigFixtureFloor = 8

// committedConfigFixtures walks the module for the fixture NAMING convention rather than for
// a path list. Generated configs are excluded by directory: every scratch and output tree the
// repo gitignores (tests/**/.tmp, tests/*/results) holds machine-specific files that a clean
// checkout does not have, so including them would make this guard's population depend on
// which suites the developer last ran.
func committedConfigFixtures(t *testing.T) []string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	var found []string
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (srccensus.ExcludedDir(path) || generatedOutputDir(filepath.Base(path))) {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.Base(path)
		if strings.HasPrefix(name, "pinchtab") && strings.HasSuffix(name, ".json") {
			found = append(found, path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("cannot walk %s, so this guard would check nothing: %v", root, walkErr)
	}
	return found
}

func generatedOutputDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "results"
}
