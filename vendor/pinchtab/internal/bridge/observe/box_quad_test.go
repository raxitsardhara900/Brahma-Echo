package observe

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/chromedp/chromedp"
)

// Two buttons that differ ONLY in border and padding. The plain one is the
// control: with no border and no padding every box-model quad coincides, which
// is why this defect survived — every existing fixture element was unstyled.
//
// box-sizing:border-box makes the declared 120x50 the BORDER box, so the content
// box is inset by border+padding on each side and is a different rectangle in
// both origin and size. A border-only fixture would not discriminate: it still
// passes against a padding-box implementation, which is why the padding is here.
const boxQuadFixtureHTML = `<body style="margin:0">
<button id="target" style="position:absolute;left:60px;top:50px;width:120px;height:50px;border:0;padding:0;margin:0;box-sizing:border-box">target</button>
<button id="thick" style="position:absolute;left:60px;top:150px;width:120px;height:50px;border:10px solid #000;padding:8px;margin:0;box-sizing:border-box">thick</button>
</body>`

// contentBoxAABB is the content quad, read through the test's OWN
// DOM.getBoxModel call. It is deliberately not a production helper: the content
// box has no consumer through this package, and reintroducing one here as a
// convenience is how the second geometry path gets written.
func contentBoxAABB(t *testing.T, ctx context.Context, backendNodeID int64) BoundingBox {
	t.Helper()

	var result json.RawMessage
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "DOM.getBoxModel", map[string]any{
			"backendNodeId": backendNodeID,
		}, &result)
	})); err != nil {
		t.Fatal(err)
	}
	var box struct {
		Model struct {
			Content []float64 `json:"content"`
		} `json:"model"`
	}
	if err := json.Unmarshal(result, &box); err != nil {
		t.Fatal(err)
	}
	q := box.Model.Content
	if len(q) < 8 {
		t.Fatalf("content quad has %d values, want 8", len(q))
	}
	return BoundingBox{X: q[0], Y: q[1], W: q[2] - q[0], H: q[5] - q[1]}
}

// buttonNodeID resolves one of the fixture's buttons by accessible name.
func buttonNodeID(t *testing.T, ctx context.Context, name string) int64 {
	t.Helper()

	rawNodes, err := FetchAXTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nodes, _ := BuildSnapshot(rawNodes, "", -1)
	for _, n := range nodes {
		if n.Role == "button" && n.Name == name && n.NodeID != 0 {
			return n.NodeID
		}
	}
	t.Fatalf("no button named %q in the snapshot (%d nodes)", name, len(nodes))
	return 0
}

// clientRectOf is the browser's own answer, and the oracle the annotate path
// uses: internal/cdptk/annotate.go builds its overlay rects from
// getBoundingClientRect, so agreeing with it here is agreeing with
// screenshot?annotate=true.
func clientRectOf(t *testing.T, ctx context.Context, id string) BoundingBox {
	t.Helper()

	var rect struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"width"`
		H float64 `json:"height"`
	}
	script := `(() => { const r = document.getElementById('` + id + `').getBoundingClientRect(); return {x: r.x, y: r.y, width: r.width, height: r.height}; })()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &rect)); err != nil {
		t.Fatal(err)
	}
	return BoundingBox{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H}
}

func annotatedBox(t *testing.T, ctx context.Context, nodeID int64) BoundingBox {
	t.Helper()

	vp, err := FetchLayout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []A11yNode{{NodeID: nodeID}}
	if err := AnnotateBounds(ctx, nodes, false, vp); err != nil {
		t.Fatal(err)
	}
	if nodes[0].BoundingBox == nil {
		t.Fatal("AnnotateBounds produced no bounding box")
	}
	return *nodes[0].BoundingBox
}

func assertSameBox(t *testing.T, label string, got, want BoundingBox) {
	t.Helper()

	const tolerance = 0.5
	off := func(a, b float64) bool { return math.Abs(a-b) > tolerance }
	if off(got.X, want.X) || off(got.Y, want.Y) || off(got.W, want.W) || off(got.H, want.H) {
		t.Errorf("%s = {x %.0f y %.0f w %.0f h %.0f}, want {x %.0f y %.0f w %.0f h %.0f}",
			label, got.X, got.Y, got.W, got.H, want.X, want.Y, want.W, want.H)
	}
}

// The card's defect: three surfaces describing the same element, two agreeing
// and /capture reporting a rectangle inset by the border. capture bounds come
// from AnnotateBounds, /box from ElementBorderBox, and annotate rects from
// getBoundingClientRect — so all three conventions are compared here, on a
// bordered+padded element and on an unstyled control.
func TestCaptureBoundsAgreeWithBoxAndClientRect(t *testing.T) {
	fixture := newScrollFixtureFrom(t, boxQuadFixtureHTML, 0, 0)

	for _, tc := range []struct {
		name string
		id   string
	}{
		{"unstyled control", "target"},
		{"bordered and padded", "thick"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodeID := buttonNodeID(t, fixture.ctx, tc.id)
			clientRect := clientRectOf(t, fixture.ctx, tc.id)

			assertSameBox(t, "capture boundingBox (AnnotateBounds)", annotatedBox(t, fixture.ctx, nodeID), clientRect)

			borderBox, ok := ElementBorderBox(fixture.ctx, nodeID)
			if !ok {
				t.Fatal("ElementBorderBox reported no box")
			}
			assertSameBox(t, "/box (ElementBorderBox)", borderBox, clientRect)
		})
	}
}

// Without this the test above could pass against a content-box implementation
// that happens to agree — it would mean the fixture cannot see the defect. The
// plain button is the other half: its quads coincide, so a change here must move
// the bordered element and leave the unstyled one exactly where it was.
func TestBoxQuadFixtureDistinguishesContentFromBorder(t *testing.T) {
	fixture := newScrollFixtureFrom(t, boxQuadFixtureHTML, 0, 0)

	thick := buttonNodeID(t, fixture.ctx, "thick")
	border, ok := ElementBorderBox(fixture.ctx, thick)
	if !ok {
		t.Fatal("ElementBorderBox reported no box for the bordered element")
	}
	content := contentBoxAABB(t, fixture.ctx, thick)

	if content.X == border.X && content.Y == border.Y {
		t.Fatalf("content and border boxes share an origin (%v); the fixture cannot tell the two quads apart", border)
	}
	if content.W == border.W || content.H == border.H {
		t.Fatalf("content %v and border %v share a dimension; a padding-box implementation would pass unnoticed", content, border)
	}
	if content.W >= border.W || content.H >= border.H {
		t.Fatalf("content %v is not inside border %v", content, border)
	}

	plain := buttonNodeID(t, fixture.ctx, "target")
	plainBorder, ok := ElementBorderBox(fixture.ctx, plain)
	if !ok {
		t.Fatal("ElementBorderBox reported no box for the unstyled element")
	}
	assertSameBox(t, "unstyled control content quad", contentBoxAABB(t, fixture.ctx, plain), plainBorder)
}
