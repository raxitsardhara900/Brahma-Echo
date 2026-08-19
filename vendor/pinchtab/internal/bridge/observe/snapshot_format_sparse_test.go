package observe

import (
	"strings"
	"testing"
)

// A filtered view hands the formatter a sparse ref set — e0, e1, e6 — because a
// ref now denotes a node rather than a row. The formatter writes each node's own
// ref verbatim; it must never renumber to a dense e0, e1, e2 run, or the printed
// ref would no longer match the ref the agent must echo back to act on the node.
func TestFormattersPreserveASparseRefSet(t *testing.T) {
	nodes := []A11yNode{
		{Ref: "e0", Role: "button", Name: "Alpha"},
		{Ref: "e1", Role: "link", Name: "Beta"},
		{Ref: "e6", Role: "textbox", Name: "Gamma"},
	}

	for _, tc := range []struct {
		name  string
		out   string
		want  string // the sparse ref that must survive
		dense string // the dense ref that must NOT appear
	}{
		{"text", FormatSnapshotText(nodes), "e6 textbox", "e2 "},
		{"compact", FormatSnapshotCompact(nodes), "e6:textbox", "e2:"},
	} {
		if !strings.Contains(tc.out, tc.want) {
			t.Errorf("%s formatter dropped the sparse ref e6:\n%s", tc.name, tc.out)
		}
		if strings.Contains(tc.out, tc.dense) {
			t.Errorf("%s formatter renumbered the sparse set to dense (found %q):\n%s", tc.name, tc.dense, tc.out)
		}
	}
}
