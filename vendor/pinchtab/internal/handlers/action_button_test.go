package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
	bridgecdpops "github.com/pinchtab/pinchtab/internal/bridge/cdpops"
	"github.com/pinchtab/pinchtab/internal/config"
)

// The CLI is not the only client: the query-parameter path accepts anything a caller sends,
// so the HTTP body has to be refused server-side rather than relying on the local flag check.
// This is the surface where an unknown button previously became a left-button action with a
// 200 and no reason.
func TestTheActionEndpointRefusesAnUnknownButtonWith400(t *testing.T) {
	h := New(&mockBridge{availableActions: []string{bridge.ActionMouseDown}}, &config.RuntimeConfig{}, nil, nil, nil)

	for _, tc := range []struct{ name, body string }{
		{name: "misspelling", body: `{"kind":"mouse-down","button":"rihgt","tabId":"tab1"}`},
		{name: "the DOM vocabulary", body: `{"kind":"mouse-down","button":"secondary","tabId":"tab1"}`},
		{name: "numeric", body: `{"kind":"mouse-down","button":"0","tabId":"tab1"}`},
	} {
		w := httptest.NewRecorder()
		h.HandleAction(w, httptest.NewRequest("POST", "/action", strings.NewReader(tc.body)))

		if w.Code != 400 {
			t.Errorf("%s: status = %d, want 400; a button nothing can honour must not reach the browser", tc.name, w.Code)
			continue
		}
		for _, name := range bridgecdpops.MouseButtons() {
			if !strings.Contains(w.Body.String(), name) {
				t.Errorf("%s: body = %s, want it to name the valid button %q", tc.name, w.Body.String(), name)
			}
		}
	}
}

// The pair assertion: refusing unknown names is satisfiable by refusing everything, which
// would break every caller that never named a button.
func TestTheActionEndpointStillAcceptsTheValidButtonsAndTheDefault(t *testing.T) {
	h := New(&mockBridge{availableActions: []string{bridge.ActionMouseDown}}, &config.RuntimeConfig{}, nil, nil, nil)

	for _, button := range append(bridgecdpops.MouseButtons(), "RIGHT", " middle ") {
		body := `{"kind":"mouse-down","button":"` + button + `","tabId":"tab1"}`
		w := httptest.NewRecorder()
		h.HandleAction(w, httptest.NewRequest("POST", "/action", strings.NewReader(body)))
		if w.Code == 400 {
			t.Errorf("button %q was refused: %s", button, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	h.HandleAction(w, httptest.NewRequest("POST", "/action", strings.NewReader(`{"kind":"mouse-down","tabId":"tab1"}`)))
	if w.Code == 400 {
		t.Errorf("an unspecified button was refused: %s", w.Body.String())
	}
}
