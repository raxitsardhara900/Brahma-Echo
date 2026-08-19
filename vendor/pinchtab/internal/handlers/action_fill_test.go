package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

// fillCapture drives HandleAction and reports what the handler forwarded, so the two
// requests that look identical inside the bridge — clear this field, and the text never
// arrived — can be told apart at the boundary that still knows.
func fillCapture(t *testing.T, body string) (*submitActionBridge, int, string, bridge.ActionRequest) {
	t.Helper()

	parentCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	b := newSubmitActionBridge(parentCtx)
	b.availableActions = []string{bridge.ActionFill}
	var forwarded bridge.ActionRequest
	b.executeActionFn = func(_ context.Context, _ string, req bridge.ActionRequest) (map[string]any, error) {
		forwarded = req
		text, _ := bridge.FillText(req)
		return map[string]any{"filled": true, "len": len(text)}, nil
	}

	h := New(b, &config.RuntimeConfig{ActionTimeout: time.Second}, nil, nil, nil)
	w := postSubmitRequest(t, h, body)
	return b, w.Code, w.Body.String(), forwarded
}

// The silent no-op, closed at the handler rather than in the action: the ghost-chrome proxy
// answers fill from its static browser before the Chrome action runs, so a check that lived
// only in actionFill would still be bypassed for that provider.
func TestHandleActionRefusesAFillCarryingNoTextAtAll(t *testing.T) {
	b, code, body, _ := fillCapture(t, `{"kind":"fill","tabId":"tab1","nodeId":42}`)

	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; this request writes nothing and used to answer filled:true with len:0\n%s", code, body)
	}
	if !strings.Contains(body, "missing_fill_text") {
		t.Errorf("body = %s, want the missing_fill_text code", body)
	}
	if b.actionCalls != 0 {
		t.Errorf("the refused fill still reached the browser %d time(s)", b.actionCalls)
	}
}

// Clearing a field is a real operation and must survive the refusal above, which is the
// whole reason presence is carried separately from emptiness.
func TestHandleActionStillClearsOnAnExplicitEmptyText(t *testing.T) {
	b, code, body, forwarded := fillCapture(t, `{"kind":"fill","tabId":"tab1","nodeId":42,"text":""}`)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for an explicit clear\n%s", code, body)
	}
	if b.actionCalls != 1 {
		t.Fatalf("action calls = %d, want the clear to be executed", b.actionCalls)
	}
	text, supplied := bridge.FillText(forwarded)
	if !supplied || text != "" {
		t.Errorf("forwarded FillText() = (%q, %v), want the empty clear to arrive as supplied", text, supplied)
	}
}

// The raw HTTP API is public and nothing documented which spelling fill reads, so a caller
// POSTing value gets the write rather than a silent empty one.
func TestHandleActionFillAcceptsValueAsWellAsText(t *testing.T) {
	b, code, body, forwarded := fillCapture(t, `{"kind":"fill","tabId":"tab1","nodeId":42,"value":"FROM_VALUE"}`)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", code, body)
	}
	if b.actionCalls != 1 {
		t.Fatalf("action calls = %d, want the fill to be executed", b.actionCalls)
	}
	if text, _ := bridge.FillText(forwarded); text != "FROM_VALUE" {
		t.Errorf("forwarded FillText() = %q, want the value the caller supplied", text)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("response is not JSON (%v): %s", err, body)
	}
}
