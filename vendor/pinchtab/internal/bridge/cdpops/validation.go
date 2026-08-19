package cdpops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"
)

var (
	ErrElementOccluded  = errors.New("element is occluded")
	ErrElementHidden    = errors.New("element is hidden")
	ErrElementBlocked   = errors.New("element is blocked from pointer interaction")
	ErrElementOffscreen = errors.New("element center is outside viewport")
)

type pointerProbe struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
	InViewport   bool    `json:"inViewport"`
	Visible      bool    `json:"visible"`
	PointerEvent string  `json:"pointerEvent"`
	Occluded     bool    `json:"occluded"`
	TopTag       string  `json:"topTag"`
}

// PointerPointForNode validates clickability assumptions and returns a
// stable pointer coordinate for the backend node.
func PointerPointForNode(ctx context.Context, backendNodeID int64, requireTopMost bool) (float64, float64, error) {
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "DOM.scrollIntoViewIfNeeded", map[string]any{"backendNodeId": backendNodeID}, nil)
	})); err != nil {
		return 0, 0, fmt.Errorf("scroll into view: %w", err)
	}

	objectID, err := IsolatedNodeObjectID(ctx, backendNodeID)
	if err != nil {
		return 0, 0, err
	}

	// view/doc come from the node, never from the ambient globals. In the main
	// world those globals were the node's own frame; in an isolated world the
	// handle carries no such guarantee, and reading them there would compute the
	// style, the frame walk, the viewport bounds and the occlusion hit-test
	// against the top frame for a node that lives in an iframe.
	const probeJS = `function() {
		const view = (this.ownerDocument && this.ownerDocument.defaultView) || window;
		const doc = this.ownerDocument || document;
		const r = this.getBoundingClientRect();
		const style = view.getComputedStyle(this);
		const localX = r.left + (r.width / 2);
		const localY = r.top + (r.height / 2);
		let x = localX;
		let y = localY;
		let topWindow = view;
		try {
			let current = view;
			while (current && current.parent && current !== current.parent) {
				const frameEl = current.frameElement;
				if (!frameEl) {
					break;
				}
				const frameRect = frameEl.getBoundingClientRect();
				x += frameRect.left;
				y += frameRect.top;
				current = current.parent;
				topWindow = current;
			}
		} catch (e) {
			// Cross-origin ancestors can block frame traversal. In that case we keep
			// the frame-local coordinates and let higher layers decide whether that
			// target is safely actionable.
		}
		const viewportWidth = topWindow && topWindow.innerWidth ? topWindow.innerWidth : view.innerWidth;
		const viewportHeight = topWindow && topWindow.innerHeight ? topWindow.innerHeight : view.innerHeight;
		const inViewport = x >= 0 && y >= 0 && x <= viewportWidth && y <= viewportHeight;
		const visible = !!style && style.display !== 'none' && style.visibility !== 'hidden' && Number(style.opacity || '1') > 0;
		const pointerEvent = style ? String(style.pointerEvents || '') : '';
		let occluded = false;
		let topTag = '';
		if (localX >= 0 && localY >= 0 && localX <= view.innerWidth && localY <= view.innerHeight) {
			const top = doc.elementFromPoint(localX, localY);
			if (top) {
				topTag = String(top.tagName || '').toLowerCase();
				const related = top === this || this.contains(top) || top.contains(this);
				occluded = !related;
			}
		}
		return {
			x,
			y,
			width: r.width,
			height: r.height,
			inViewport,
			visible,
			pointerEvent,
			occluded,
			topTag
		};
	}`

	var probeRaw json.RawMessage
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "Runtime.callFunctionOn", map[string]any{
			"functionDeclaration": probeJS,
			"objectId":            objectID,
			"returnByValue":       true,
		}, &probeRaw)
	})); err != nil {
		return 0, 0, fmt.Errorf("pointer probe: %w", err)
	}

	var callRes struct {
		Result struct {
			Value pointerProbe `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(probeRaw, &callRes); err != nil {
		return 0, 0, err
	}
	probe := callRes.Result.Value

	if probe.Width <= 0 || probe.Height <= 0 || !probe.Visible {
		return 0, 0, fmt.Errorf("%w: width=%.2f height=%.2f", ErrElementHidden, probe.Width, probe.Height)
	}
	if !probe.InViewport {
		return 0, 0, fmt.Errorf("%w: x=%.2f y=%.2f", ErrElementOffscreen, probe.X, probe.Y)
	}
	if strings.EqualFold(strings.TrimSpace(probe.PointerEvent), "none") {
		return 0, 0, fmt.Errorf("%w: pointer-events=none", ErrElementBlocked)
	}
	if requireTopMost && probe.Occluded {
		return 0, 0, fmt.Errorf("%w: top=%s", ErrElementOccluded, probe.TopTag)
	}

	return probe.X, probe.Y, nil
}
