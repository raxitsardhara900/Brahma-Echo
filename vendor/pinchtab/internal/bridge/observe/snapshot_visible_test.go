package observe

import (
	"encoding/json"
	"testing"
)

func marshalNode(t *testing.T, node A11yNode) map[string]any {
	t.Helper()
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal node: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode node: %v", err)
	}
	return decoded
}

// assertVisiblePairsWithBounds is the invariant the whole card rests on: the
// wire says "visible" exactly when it says "boundingBox", so absence of the key
// means "not measured" and nothing else.
func assertVisiblePairsWithBounds(t *testing.T, nodes []map[string]any) {
	t.Helper()
	if len(nodes) == 0 {
		t.Fatal("no nodes to check the visible/boundingBox invariant over")
	}
	for i, node := range nodes {
		_, hasVisible := node["visible"]
		_, hasBounds := node["boundingBox"]
		if hasVisible != hasBounds {
			t.Errorf("node %d (%v): visible present=%v but boundingBox present=%v", i, node["ref"], hasVisible, hasBounds)
		}
	}
}

// A measured node that sits off-screen must serialise its false, which is the
// value omitempty on a plain bool erased.
func TestMeasuredOffScreenNodeSerialisesVisibleFalse(t *testing.T) {
	offScreen := false
	node := marshalNode(t, A11yNode{Ref: "e1", BoundingBox: &BoundingBox{Y: -2410, W: 80, H: 20}, Visible: &offScreen})

	value, ok := node["visible"]
	if !ok {
		t.Fatalf("a measured off-screen node lost its visible key: %v", node)
	}
	if value != false {
		t.Fatalf("visible = %v, want false", value)
	}
}

func TestUnmeasuredNodeSerialisesWithoutVisibleKey(t *testing.T) {
	node := marshalNode(t, A11yNode{Ref: "e1"})
	if _, ok := node["visible"]; ok {
		t.Fatalf("an unmeasured node published a visible key: %v", node)
	}

	onScreen := true
	measured := marshalNode(t, A11yNode{Ref: "e2", BoundingBox: &BoundingBox{W: 80, H: 20}, Visible: &onScreen})
	if measured["visible"] != true {
		t.Fatalf("visible = %v, want true", measured["visible"])
	}

	assertVisiblePairsWithBounds(t, []map[string]any{node, measured})
}

// The other three bools come from the accessibility tree for every node, so for
// them absent already means false and omitempty is correct. Converting them for
// symmetry would widen the wire change for no gain, so the tags are pinned.
func TestAccessibilityTreeBoolsKeepOmitempty(t *testing.T) {
	node := marshalNode(t, A11yNode{Ref: "e1"})
	for _, key := range []string{"disabled", "focused", "hidden"} {
		if _, ok := node[key]; ok {
			t.Errorf("%q must stay omitempty: a false value belongs off the wire", key)
		}
	}

	enabled := marshalNode(t, A11yNode{Ref: "e1", Disabled: true, Focused: true, Hidden: true})
	for _, key := range []string{"disabled", "focused", "hidden"} {
		if enabled[key] != true {
			t.Errorf("%q = %v, want true", key, enabled[key])
		}
	}
}
