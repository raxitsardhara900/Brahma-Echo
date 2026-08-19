package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type importSource struct {
	root        *os.Root
	relative    string
	displayPath string
}

func openImportSource(sourcePath string) (*importSource, error) {
	if sourcePath == "" {
		return nil, fmt.Errorf("source path required")
	}

	cleaned := filepath.Clean(sourcePath)
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return nil, fmt.Errorf("source path invalid: %w", err)
		}
		cleaned = abs
	}

	roots, err := allowedImportRoots()
	if err != nil {
		return nil, err
	}
	for _, rootPath := range roots {
		relative, err := filepath.Rel(rootPath, cleaned)
		if err != nil || !pathWithinRoot(cleaned, rootPath) {
			continue
		}
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			return nil, fmt.Errorf("open import root: %w", err)
		}
		return &importSource{root: root, relative: relative, displayPath: cleaned}, nil
	}
	return nil, fmt.Errorf("source path must be within %s", strings.Join(roots, " or "))
}

func (s *importSource) child(name string) string {
	if s.relative == "." {
		return name
	}
	return filepath.Join(s.relative, name)
}

func allowedImportRoots() ([]string, error) {
	roots := []string{filepath.Clean(os.TempDir())}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	roots = append(roots, filepath.Clean(homeDir))
	return roots, nil
}

func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}
