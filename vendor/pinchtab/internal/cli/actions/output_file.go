package actions

import (
	"fmt"
	"os"

	"github.com/pinchtab/pinchtab/internal/cli"
	"github.com/pinchtab/pinchtab/internal/fileout"
)

// writeOutputFile writes buf to path and returns the path actually used.
//
// An auto-named path is reserved with an exclusive create, so a second run in the same
// second gets capture-<ts>-1.jpg rather than destroying the first file. The returned
// path is what the caller must print: it may carry a suffix the caller did not build,
// and printing the built name instead trades a silent overwrite for a confidently
// reported filename that does not exist.
//
// An explicit path is written as the user typed it, overwrite included — that is their
// instruction, not a collision.
func writeOutputFile(path string, autoNamed bool, buf []byte) (string, error) {
	if autoNamed {
		return fileout.WriteUniquePath(path, buf)
	}
	return path, os.WriteFile(path, buf, 0600)
}

// capture's lowercase "saved" and record's "Saved → %s" are deliberately NOT routed
// here. They are different sentences for different shapes — capture prints a save line
// alongside the snapshot that is half of what it returned, and record has no byte count
// because it moves an already-encoded file — so folding them in would change their
// output rather than remove a duplicate.
func printSaved(path string, size int) {
	fmt.Println(cli.StyleStdout(cli.SuccessStyle, fmt.Sprintf("Saved %s (%d bytes)", path, size)))
}
