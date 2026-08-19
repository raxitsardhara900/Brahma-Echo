package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirRejectsDestinationSymlinkEscape(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(srcPath, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcPath, "Default", "Preferences"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := openImportSource(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.root.Close() }()

	dst := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dst, "Default")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	dstRoot, err := os.OpenRoot(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dstRoot.Close() }()

	if err := copyDir(src, dstRoot); err == nil {
		t.Fatal("copyDir followed a destination symlink outside its root")
	}
	if _, err := os.Stat(filepath.Join(outside, "Preferences")); !os.IsNotExist(err) {
		t.Fatalf("copyDir wrote outside its destination root: %v", err)
	}
}
