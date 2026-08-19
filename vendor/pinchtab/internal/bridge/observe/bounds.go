package observe

import (
	"context"
	"encoding/json"

	"github.com/chromedp/chromedp"
)

type ViewportInfo struct {
	Width            float64
	Height           float64
	ScrollX          float64
	ScrollY          float64
	DevicePixelRatio float64
}

// One CDP round trip via Runtime.evaluate so this composes inside PairedCapture
// without an extra Page.getLayoutMetrics call.
func FetchLayout(ctx context.Context) (ViewportInfo, error) {
	var out ViewportInfo
	const expression = `JSON.stringify({
		w: window.innerWidth,
		h: window.innerHeight,
		sx: window.scrollX || window.pageXOffset || 0,
		sy: window.scrollY || window.pageYOffset || 0,
		dpr: window.devicePixelRatio || 1
	})`
	var result struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "Runtime.evaluate", map[string]any{
			"expression":    expression,
			"returnByValue": true,
		}, &result)
	})); err != nil {
		return out, err
	}
	var parsed struct {
		W   float64 `json:"w"`
		H   float64 `json:"h"`
		SX  float64 `json:"sx"`
		SY  float64 `json:"sy"`
		DPR float64 `json:"dpr"`
	}
	if err := json.Unmarshal([]byte(result.Result.Value), &parsed); err != nil {
		return out, err
	}
	out.Width = parsed.W
	out.Height = parsed.H
	out.ScrollX = parsed.SX
	out.ScrollY = parsed.SY
	out.DevicePixelRatio = parsed.DPR
	return out, nil
}

// Each node costs one DOM.getBoxModel round trip; for the typical
// FilterInteractive snapshot of <50 nodes the total budget is ~250ms.
//
// DOM.getBoxModel returns CSS coordinates relative to the MAIN FRAME'S VIEWPORT
// — measured against getBoundingClientRect on a scrolled page, the two agree
// exactly — which is what makes it correct for nodes inside iframes, where a
// rect taken in the element's own context is frame-relative.
//
// pageCoords=false leaves boxes in that viewport space. pageCoords=true adds
// the scroll offset to reach document space, which is what beyondViewport and
// clip captures report and what projectBoundsToClip subtracts a clip origin
// from — that origin is document-relative, so both sides must be.
//
// Visibility heuristic: a node is Visible if its rect has non-zero area and
// intersects the viewport. The check is intentionally cheap — strict
// occlusion (document.elementFromPoint) is deferred.
func AnnotateBounds(ctx context.Context, nodes []A11yNode, pageCoords bool, vp ViewportInfo) error {
	for i := range nodes {
		if nodes[i].NodeID == 0 {
			continue
		}
		box, ok := ElementBorderBox(ctx, nodes[i].NodeID)
		if !ok {
			continue
		}
		visible := IsOnScreen(box, vp)
		if pageCoords {
			box.X += vp.ScrollX
			box.Y += vp.ScrollY
		}
		nodes[i].BoundingBox = &box
		nodes[i].Visible = &visible
	}
	return nil
}

// ElementBorderBox is the border-box rectangle of a node in viewport-relative
// CSS coordinates, the space getBoxModel reports and the space /box and
// ScrollIntoViewAndGetBox hand to their callers unchanged. It is the
// cross-frame-correct alternative to evaluating getBoundingClientRect in the
// element's own context, which is relative to the document the element lives in
// and therefore frame-relative for anything inside an iframe.
//
// Every bounds consumer goes through here. AnnotateBounds used to take
// getBoxModel's CONTENT quad, which meant /capture reported a rectangle inset by
// each element's border and padding while /box and the annotate path reported the
// border box — the same field name, the same origin, a different box-model edge.
// Overlays drawn from it sat inside the painted border. The content quad has no
// consumer through this helper: the click sites read box.Content from their own
// CDP calls in internal/bridge/input_human.go and cdpops/geometry.go.
func ElementBorderBox(ctx context.Context, backendNodeID int64) (BoundingBox, bool) {
	var result json.RawMessage
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "DOM.getBoxModel", map[string]any{
			"backendNodeId": backendNodeID,
		}, &result)
	}))
	if err != nil {
		return BoundingBox{}, false
	}
	var box struct {
		Model struct {
			Border []float64 `json:"border"`
		} `json:"model"`
	}
	if err := json.Unmarshal(result, &box); err != nil {
		return BoundingBox{}, false
	}
	q := box.Model.Border
	if len(q) < 8 {
		return BoundingBox{}, false
	}
	// AABB across the 4 corners — robust against transformed elements.
	minX, maxX := q[0], q[0]
	minY, maxY := q[1], q[1]
	for k := 2; k < 8; k += 2 {
		if q[k] < minX {
			minX = q[k]
		}
		if q[k] > maxX {
			maxX = q[k]
		}
		if q[k+1] < minY {
			minY = q[k+1]
		}
		if q[k+1] > maxY {
			maxY = q[k+1]
		}
	}
	return BoundingBox{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}, true
}

// IsOnScreen is the one owner of the on-screen question: positive area AND
// intersection with the viewport. b is in viewport coordinates, the space
// DOM.getBoxModel reports and the space AnnotateBounds measures in before any
// document transform. It is deliberately NOT the rendered-ness question that
// GET /visible answers — that predicate ignores scroll position.
func IsOnScreen(b BoundingBox, vp ViewportInfo) bool {
	if b.W <= 0 || b.H <= 0 {
		return false
	}
	if vp.Width <= 0 || vp.Height <= 0 {
		return true
	}
	return b.X+b.W > 0 &&
		b.Y+b.H > 0 &&
		b.X < vp.Width &&
		b.Y < vp.Height
}
