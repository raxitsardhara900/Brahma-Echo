package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/pinchtab/pinchtab/internal/bridge"
)

var ErrElementNotFound = errors.New("element not found")

// ErrSemanticMatcherUnavailable is the one server fault in the semantic selector path:
// the feature is not configured, which is not the caller's mistake and not a missing
// element. It carries its own sentinel so the mapping below can keep answering 501 for
// it without asking what the message says.
var ErrSemanticMatcherUnavailable = errors.New("semantic selectors require a matcher (not configured)")

func (h *Handlers) resolveElementNodeID(ctx context.Context, tabID, sel string) (int64, error) {
	nodeID, err := h.resolveSelectorNodeID(ctx, tabID, sel)
	if err != nil {
		// Only a genuine "selector matched no element" is a 404. CDP/transport
		// faults, unsupported selector kinds, and internal routing errors must
		// stay 5xx so real bridge failures don't masquerade as a missing element.
		if errors.Is(err, bridge.ErrSelectorNoMatch) {
			return 0, fmt.Errorf("%w: %q: %v", ErrElementNotFound, sel, err)
		}
		return 0, err
	}
	if nodeID == 0 {
		return 0, fmt.Errorf("%w: %q", ErrElementNotFound, sel)
	}
	return nodeID, nil
}

// statusForElementErr is the ONE owner of the selector-failure status, for the inspect
// endpoints and the action path alike. It reads SENTINELS, never message text: a status
// keyed on wording changes the moment someone rewrites a sentence, and the semantic
// messages are rewritten often — the out-of-range one was reworded while this mapping
// was being unified.
//
// A selector that matched nothing is the caller's problem (404: re-snapshot and pick
// another). Everything else is the server's (500: retrying may help). Getting that
// backwards on a read endpoint sends retry storms at a page that is fine.
func statusForElementErr(err error) int {
	switch {
	case errors.Is(err, ErrSemanticMatcherUnavailable):
		return http.StatusNotImplemented
	case errors.Is(err, ErrElementNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
