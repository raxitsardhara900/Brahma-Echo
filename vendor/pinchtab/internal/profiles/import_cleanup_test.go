package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A live Chrome user data dir contains Singleton* symlinks, which copyDir
// deliberately refuses. The copy fails partway through, so Import must not
// leave the half-copied destination behind — otherwise the profile name is
// permanently blocked by a directory-collision error on every retry.
func TestImportCleansUpPartialCopyOnFailure(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "Preferences"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	// Sorts after "Preferences", so a chunk of the profile is copied first.
	if err := os.Symlink(filepath.Join(src, "Preferences"), filepath.Join(src, "SingletonLock")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	base := t.TempDir()
	pm := NewProfileManager(base)

	if err := pm.Import("work", src); err == nil {
		t.Fatal("Import should fail on the Singleton symlink")
	}

	dest := filepath.Join(base, profileID("work"))
	if _, err := os.Stat(dest); err == nil {
		t.Errorf("failed import left a partial profile directory at %s", dest)
	}

	retryErr := pm.Import("work", src)
	if retryErr == nil {
		t.Fatal("retry should fail the same way")
	}
	if errors.Is(retryErr, ErrProfileDirExists) || errors.Is(retryErr, ErrProfileExists) {
		t.Errorf("retry blocked by leftover state instead of reporting the real cause: %v", retryErr)
	}
}

// A successful import must still land, so the cleanup cannot be over-eager.
func TestImportSucceedsAndIsRetryableAfterDelete(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "Default"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Default", "Preferences"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	base := t.TempDir()
	pm := NewProfileManager(base)

	if err := pm.Import("work", src); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, profileID("work"), "Default", "Preferences")); err != nil {
		t.Fatalf("imported profile missing copied content: %v", err)
	}

	if err := pm.Delete("work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := pm.Import("work", src); err != nil {
		t.Fatalf("re-import after delete: %v", err)
	}
}
