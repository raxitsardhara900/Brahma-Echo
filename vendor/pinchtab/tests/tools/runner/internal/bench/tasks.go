package bench

import (
	"fmt"
	"io"
	"sort"
)

type GroupSelection struct {
	Selected []int
	Warnings []string
}

func SelectGroups(args Args, idx IndexFile, stderr io.Writer) (GroupSelection, error) {
	var sel GroupSelection

	if len(args.Groups) > 0 {
		for _, g := range args.Groups {
			sel.Selected = append(sel.Selected, g)
			if idx.IsDisabled(g) {
				w := fmt.Sprintf("warning: group %d is disabled in index but explicitly requested", g)
				sel.Warnings = append(sel.Warnings, w)
				_, _ = fmt.Fprintln(stderr, w)
			}
		}
		sort.Ints(sel.Selected)
		return sel, nil
	}

	profileGroups := resolveProfile(args.Profile)
	activeSet := make(map[int]struct{})
	for _, g := range idx.Active {
		activeSet[g] = struct{}{}
	}

	if len(profileGroups) > 0 {
		for _, g := range profileGroups {
			if _, ok := activeSet[g]; ok {
				sel.Selected = append(sel.Selected, g)
			}
		}
		sort.Ints(sel.Selected)
		return sel, nil
	}

	if len(idx.Active) > 0 {
		sel.Selected = append(sel.Selected, idx.Active...)
		return sel, nil
	}

	fallback, err := ScanGroupFiles(idx.Dir)
	if err != nil {
		return sel, fmt.Errorf("fallback scan: %w", err)
	}
	sel.Selected = fallback
	return sel, nil
}

func resolveProfile(profile string) []int {
	switch profile {
	case "common10":
		return []int{0, 1, 2, 3}
	default:
		return nil
	}
}

func GroupFilePath(dir string, group int) string {
	return fmt.Sprintf("%s/group-%02d.md", dir, group)
}
