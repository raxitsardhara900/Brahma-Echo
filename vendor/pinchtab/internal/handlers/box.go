package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pinchtab/pinchtab/internal/bridge"
)

type boundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

type boxResponse struct {
	Ref string      `json:"ref"`
	Box boundingBox `json:"box"`
}

// HandleGetBox returns the bounding box of an element identified by a unified
// selector (ref/css/xpath/text/semantic).
//
// @Endpoint GET /box
func (h *Handlers) HandleGetBox(w http.ResponseWriter, r *http.Request) {
	h.serveElementInspection(w, r, "inspect.box", func(ctx context.Context, tabID, sel string) (any, error) {
		box, err := h.getElementBox(ctx, tabID, sel)
		if err != nil {
			return nil, err
		}
		return boxResponse{Ref: sel, Box: *box}, nil
	})
}

// HandleTabGetBox returns the bounding box for a tab identified by path ID.
//
// @Endpoint GET /tabs/{id}/box
func (h *Handlers) HandleTabGetBox(w http.ResponseWriter, r *http.Request) {
	h.withPathTabID(w, r, h.HandleGetBox)
}

// getElementBox resolves a unified selector to a DOM node and returns its
// border box in top-level viewport coordinates.
//
// It measures through the same DOM.getBoxModel path the capture path uses, which
// reports every node in the top-level frame's coordinate space regardless of
// which frame the node lives in (measured: identical to a top-level
// getBoundingClientRect, scrolled or not). Evaluating getBoundingClientRect in
// the element's own context — the obvious alternative — is relative to the
// document the element lives in, so an element inside an iframe came back
// frame-relative and its rectangle landed outside the iframe containing it.
func (h *Handlers) getElementBox(ctx context.Context, tabID, sel string) (*boundingBox, error) {
	nodeID, err := h.resolveElementNodeID(ctx, tabID, sel)
	if err != nil {
		return nil, err
	}

	box, ok := bridge.ElementBorderBox(ctx, nodeID)
	if !ok {
		return nil, fmt.Errorf("element has no box model: %s", sel)
	}

	return &boundingBox{
		X:      box.X,
		Y:      box.Y,
		Width:  box.W,
		Height: box.H,
		Top:    box.Y,
		Right:  box.X + box.W,
		Bottom: box.Y + box.H,
		Left:   box.X,
	}, nil
}
