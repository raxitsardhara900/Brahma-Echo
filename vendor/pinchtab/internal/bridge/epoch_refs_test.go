package bridge

import "testing"

func refOf(nodes []A11yNode, nodeID int64) string {
	for i := range nodes {
		if nodes[i].NodeID == nodeID {
			return nodes[i].Ref
		}
	}
	return ""
}

// The core guarantee: a ref denotes a node, not a row. The same node keeps its
// ref across a filtered, scoped or depth-limited re-read, and a filtered view is
// therefore SPARSE — it returns the surviving nodes' original refs (e1, e3), not
// a fresh dense run (e0, e1). The vocabulary token is carried, so the superseded
// guard stays silent for the reader that changed only its filter.
func TestARefSurvivesAChangeOfFilterAndTheFilteredViewIsSparse(t *testing.T) {
	full := []A11yNode{{NodeID: 10}, {NodeID: 20}, {NodeID: 30}, {NodeID: 40}}
	c1 := EpochRefs(nil, full)

	want := map[int64]string{10: "e0", 20: "e1", 30: "e2", 40: "e3"}
	for id, ref := range want {
		if got := refOf(full, id); got != ref {
			t.Fatalf("first read: node %d ref = %q, want %q", id, got, ref)
		}
	}

	// A filtered / scoped re-read that keeps only two of the four nodes, in a
	// different slice, must return their ORIGINAL refs.
	filtered := []A11yNode{{NodeID: 40}, {NodeID: 20}}
	c2 := EpochRefs(c1, filtered)
	if got := refOf(filtered, 20); got != "e1" {
		t.Errorf("node 20 ref after filter = %q, want e1 — a ref must denote the node, not its row", got)
	}
	if got := refOf(filtered, 40); got != "e3" {
		t.Errorf("node 40 ref after filter = %q, want e3", got)
	}
	if _, dense := c2.Refs["e0"]; dense {
		t.Errorf("filtered view is dense (e0 present); it must be sparse: %v", c2.Refs)
	}
	if c2.DomEpoch != c1.DomEpoch {
		t.Errorf("vocabulary token changed on a filter-only re-read (%q -> %q); the superseded guard would fire spuriously", c1.DomEpoch, c2.DomEpoch)
	}
}

// A genuinely new document — no backend node id shared with the prior read —
// starts a fresh epoch: refs renumber from e0 and a new vocabulary token is
// minted, so an action carrying a stale ref+token is refused loudly rather than
// resolved against the wrong node.
func TestANewDocumentStartsAFreshEpochAndSupersedesTheToken(t *testing.T) {
	c1 := EpochRefs(nil, []A11yNode{{NodeID: 10}, {NodeID: 20}})
	newDoc := []A11yNode{{NodeID: 99}, {NodeID: 88}}
	c2 := EpochRefs(c1, newDoc)

	if c2.DomEpoch == c1.DomEpoch {
		t.Error("a new document kept the prior vocabulary token; the superseded guard could not fire")
	}
	if got := refOf(newDoc, 99); got != "e0" {
		t.Errorf("new epoch first node ref = %q, want e0", got)
	}
}

// A statically fetched page carries no backend identity: every node has id zero
// and its ref is minted by the static browser, which keeps it stable per element.
// EpochRefs must PRESERVE those refs — renumbering them positionally would put
// the published ref out of step with the static browser's own ref->element map
// and resolve a later click to the wrong element.
func TestStaticRefsArePreservedNotRenumbered(t *testing.T) {
	// A sparse, out-of-order static ref set, exactly as a filtered static re-read
	// produces it.
	static := []A11yNode{{Ref: "e5", NodeID: 0}, {Ref: "e2", NodeID: 0}}
	c := EpochRefs(nil, static)

	if static[0].Ref != "e5" || static[1].Ref != "e2" {
		t.Fatalf("static refs were renumbered to %q,%q; they must be preserved verbatim", static[0].Ref, static[1].Ref)
	}
	if len(c.Refs) != 0 {
		t.Errorf("zero-id static nodes were made resolvable via the handler cache: %v", c.Refs)
	}
	if _, ok := c.Lookup("e5"); ok {
		t.Error("a zero-id static ref resolved through the handler cache; it must resolve through the static browser only")
	}

	// A second static read (different subset) still leaves the refs alone.
	static2 := []A11yNode{{Ref: "e2", NodeID: 0}}
	EpochRefs(c, static2)
	if static2[0].Ref != "e2" {
		t.Errorf("static ref renumbered on a re-read to %q, want e2", static2[0].Ref)
	}
}

// The ghost-chrome escalation seam publishes a cache with only Refs (static ref
// string -> Chrome backend node id) and no Book. The next Chrome read must seed
// its book from those Refs so the node the agent already holds as e3 keeps e3
// after escalation, rather than being renumbered — the instability this whole
// change exists to remove, reintroduced at the seam.
func TestEscalationSeamPreservesTheStaticRefString(t *testing.T) {
	escalation := &RefCache{Refs: map[string]int64{"e3": 100}}
	chromeNodes := []A11yNode{{NodeID: 100}, {NodeID: 200}}
	c := EpochRefs(escalation, chromeNodes)

	if got := refOf(chromeNodes, 100); got != "e3" {
		t.Errorf("node 100 ref after escalation = %q, want e3 — the static ref must survive the transition to Chrome", got)
	}
	if c.Refs["e3"] != 100 {
		t.Errorf("ref e3 no longer resolves to node 100 after escalation: %v", c.Refs)
	}
	if got := refOf(chromeNodes, 200); got == "e3" {
		t.Error("a newly seen node collided with the carried static ref e3")
	}
}

// A zero-id node mixed in among resolvable nodes has no identity to key on, so it
// gets a fresh positional ref every read and is never resolvable. That instability
// is harmless precisely because such a ref never enters the resolvable set.
func TestAZeroIdNodeAmongResolvableNodesIsNeverResolvable(t *testing.T) {
	mixed := []A11yNode{{NodeID: 10}, {NodeID: 0}, {NodeID: 20}}
	c := EpochRefs(nil, mixed)

	zeroRef := mixed[1].Ref
	if zeroRef == "" {
		t.Fatal("the zero-id node was not stamped with a ref")
	}
	if _, ok := c.Refs[zeroRef]; ok {
		t.Errorf("the zero-id node's ref %q is resolvable; a node with no backend id must never resolve", zeroRef)
	}
	if c.Refs["e0"] != 10 || c.Refs["e2"] != 20 {
		t.Errorf("resolvable nodes lost their refs among a zero-id node: %v", c.Refs)
	}
}
