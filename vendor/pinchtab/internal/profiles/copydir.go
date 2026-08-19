package profiles

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func copyDir(src *importSource, dst *os.Root) error {
	return fs.WalkDir(src.root.FS(), filepath.ToSlash(src.relative), func(sourcePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in imported profiles: %s", sourcePath)
		}

		rel, err := filepath.Rel(filepath.FromSlash(src.relative), filepath.FromSlash(sourcePath))
		if err != nil {
			return err
		}
		target := filepath.FromSlash(rel)
		if !filepath.IsLocal(target) {
			return fmt.Errorf("import path escapes destination: %s", sourcePath)
		}

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return dst.MkdirAll(target, info.Mode().Perm())
		}

		return copyFile(src.root, filepath.FromSlash(sourcePath), dst, target)
	})
}

func copyFile(srcRoot *os.Root, src string, dstRoot *os.Root, dst string) error {
	in, err := srcRoot.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := dstRoot.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return out.Close()
}
