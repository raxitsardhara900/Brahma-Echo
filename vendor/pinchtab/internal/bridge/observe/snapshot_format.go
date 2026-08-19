package observe

import "strings"

// All THREE states are annotated, not just the checked one: absent means the control
// has no checkedness and false means it is off, so rendering only [checked] would make
// an unchecked option look like a node the field does not apply to.
var checkedAnnotations = map[CheckedState]string{
	CheckedTrue:  " [checked]",
	CheckedFalse: " [unchecked]",
	CheckedMixed: " [mixed]",
}

// [~] is NOT available here: the compact diff renderer already uses it for
// "changed", and the same token meaning two things on one line is unreadable.
var checkedCompactAnnotations = map[CheckedState]string{
	CheckedTrue:  " [x]",
	CheckedFalse: " [ ]",
	CheckedMixed: " [/]",
}

func FormatSnapshotText(nodes []A11yNode) string {
	var b strings.Builder
	for _, n := range nodes {
		for i := 0; i < n.Depth; i++ {
			b.WriteString("  ")
		}
		b.WriteString(n.Ref)
		b.WriteByte(' ')
		b.WriteString(n.Role)
		if n.Name != "" {
			b.WriteString(` "`)
			b.WriteString(n.Name)
			b.WriteByte('"')
		}
		if n.Value != "" {
			b.WriteString(` val="`)
			b.WriteString(n.Value)
			b.WriteByte('"')
		}
		if n.Focused {
			b.WriteString(" [focused]")
		}
		if annotation, ok := checkedAnnotations[n.Checked]; ok {
			b.WriteString(annotation)
		}
		if n.Disabled {
			b.WriteString(" [disabled]")
		}
		if n.Hidden {
			b.WriteString(" [hidden]")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func FormatSnapshotCompact(nodes []A11yNode) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.Ref)
		b.WriteByte(':')
		b.WriteString(n.Role)
		if n.Name != "" {
			b.WriteString(` "`)
			b.WriteString(n.Name)
			b.WriteByte('"')
		}
		if n.Value != "" {
			b.WriteString(` val="`)
			b.WriteString(n.Value)
			b.WriteByte('"')
		}
		if n.Focused {
			b.WriteString(" *")
		}
		if annotation, ok := checkedCompactAnnotations[n.Checked]; ok {
			b.WriteString(annotation)
		}
		if n.Disabled {
			b.WriteString(" -")
		}
		if n.Hidden {
			b.WriteString(" [hidden]")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// FormatSnapshotCompactDiff outputs all current nodes in compact format with
// change markers: [+] for added, [~] for changed. Removed refs are listed at
// the end as [- ref]. This gives agents the full valid ref set plus change info.
func FormatSnapshotCompactDiff(nodes []A11yNode, added, changed, removed []A11yNode) string {
	addedRefs := make(map[string]bool, len(added))
	for _, n := range added {
		addedRefs[n.Ref] = true
	}
	changedRefs := make(map[string]bool, len(changed))
	for _, n := range changed {
		changedRefs[n.Ref] = true
	}

	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.Ref)
		b.WriteByte(':')
		b.WriteString(n.Role)
		if n.Name != "" {
			b.WriteString(` "`)
			b.WriteString(n.Name)
			b.WriteByte('"')
		}
		if n.Value != "" {
			b.WriteString(` val="`)
			b.WriteString(n.Value)
			b.WriteByte('"')
		}
		if n.Focused {
			b.WriteString(" *")
		}
		if annotation, ok := checkedCompactAnnotations[n.Checked]; ok {
			b.WriteString(annotation)
		}
		if n.Disabled {
			b.WriteString(" -")
		}
		if n.Hidden {
			b.WriteString(" [hidden]")
		}
		if addedRefs[n.Ref] {
			b.WriteString(" [+]")
		} else if changedRefs[n.Ref] {
			b.WriteString(" [~]")
		}
		b.WriteByte('\n')
	}

	if len(removed) > 0 {
		b.WriteString("# removed:")
		for _, n := range removed {
			b.WriteByte(' ')
			b.WriteString(n.Ref)
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func TruncateToTokens(nodes []A11yNode, maxTokens int, format string) ([]A11yNode, bool) {
	tokensUsed := 0
	for i, n := range nodes {
		var nodeTokens int
		switch format {
		case "compact":
			size := len(n.Ref) + 1 + len(n.Role) + len(n.Name) + len(n.Value) + 8 + len(checkedCompactAnnotations[n.Checked])
			nodeTokens = size / 4
		case "text":
			size := n.Depth*2 + len(n.Ref) + 1 + len(n.Role) + len(n.Name) + len(n.Value) + 8 + len(checkedAnnotations[n.Checked])
			nodeTokens = size / 4
		default:
			size := len(n.Ref) + len(n.Role) + len(n.Name) + len(n.Value) + 60
			nodeTokens = size / 3
		}
		if nodeTokens < 1 {
			nodeTokens = 1
		}
		tokensUsed += nodeTokens
		if tokensUsed > maxTokens {
			return nodes[:i], true
		}
	}
	return nodes, false
}
