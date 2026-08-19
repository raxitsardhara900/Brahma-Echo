package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

type actionFailure struct {
	Code      string `json:"code"`
	Error     string `json:"error"`
	Retryable bool   `json:"retryable"`
}

func postActionBody(t *testing.T, mb *mockBridge, body string) (*httptest.ResponseRecorder, actionFailure) {
	t.Helper()
	h := New(mb, &config.RuntimeConfig{ActionTimeout: time.Second}, nil, nil, nil)
	w := httptest.NewRecorder()
	h.HandleAction(w, httptest.NewRequest("POST", "/action", strings.NewReader(body)))

	var failure actionFailure
	if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	return w, failure
}

func TestAnUnsatisfiableActionBodyIsACallerErrorNotAServerFault(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		executeErr error
		wantStatus int
		wantCode   string
		wantRetry  bool
		wantSays   string
	}{
		{
			name:       "a destination and an offset together",
			body:       `{"kind":"drag","nodeId":42,"toNodeId":43,"dragX":10,"tabId":"tab1"}`,
			executeErr: fmt.Errorf("action drag: %w", bridge.NewInvalidActionRequestError("drag takes a destination (toSelector/toNodeId/toX+toY) or an offset (dragX/dragY), not both")),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_action_request",
			wantRetry:  false,
			wantSays:   "not both",
		},
		{
			name:       "an offset drag with no offset",
			body:       `{"kind":"drag","nodeId":42,"tabId":"tab1"}`,
			executeErr: fmt.Errorf("action drag: %w", bridge.NewInvalidActionRequestError("dragX or dragY required for drag")),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_action_request",
			wantRetry:  false,
			wantSays:   "dragX or dragY",
		},
		{
			name:       "no target at all",
			body:       `{"kind":"drag","dragX":10,"tabId":"tab1"}`,
			executeErr: fmt.Errorf("action drag: %w", bridge.NewInvalidActionRequestError("need selector, ref, or nodeId")),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_action_request",
			wantRetry:  false,
			wantSays:   "need selector, ref, or nodeId",
		},
		{
			name:       "an unrecognised button name is still the handler's own 400",
			body:       `{"kind":"drag","nodeId":42,"dragX":10,"button":"rihgt","tabId":"tab1"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_mouse_button",
			wantRetry:  false,
			wantSays:   "rihgt",
		},
		{
			name:       "a genuine runtime failure is still a server fault the caller may retry",
			body:       `{"kind":"drag","nodeId":42,"dragX":10,"tabId":"tab1"}`,
			executeErr: fmt.Errorf("action drag: dispatch mouse event: websocket closed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "action_failed",
			wantRetry:  true,
			wantSays:   "websocket closed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mb := &mockBridge{
				availableActions: []string{bridge.ActionDrag},
				executeActionErr: tc.executeErr,
			}
			w, failure := postActionBody(t, mb, tc.body)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d; an unsatisfiable body must not be reported as a server fault, and a real fault must not be reported as the caller's error: %s",
					w.Code, tc.wantStatus, w.Body.String())
			}
			if failure.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", failure.Code, tc.wantCode)
			}
			if failure.Retryable != tc.wantRetry {
				t.Errorf("retryable = %v, want %v — retrying an unsatisfiable body can never succeed, and a transient fault is worth retrying", failure.Retryable, tc.wantRetry)
			}
			if !strings.Contains(w.Body.String(), tc.wantSays) {
				t.Errorf("body = %s, want it to carry %q; the useful half of the message must survive the status change", w.Body.String(), tc.wantSays)
			}
		})
	}
}

func TestTheCallerErrorArmKeysOnTheSentinelNotTheMessage(t *testing.T) {
	mb := &mockBridge{
		availableActions: []string{bridge.ActionDrag},
		executeActionErr: fmt.Errorf("action drag: dragX or dragY required for drag"),
	}
	w, failure := postActionBody(t, mb, `{"kind":"drag","nodeId":42,"tabId":"tab1"}`)

	if w.Code != http.StatusInternalServerError || failure.Code != "action_failed" {
		t.Fatalf("status %d code %q for an unwrapped error carrying the same wording; the handler is matching the MESSAGE, so any action whose runtime failure happens to read like a body refusal would be mis-reported as a caller error",
			w.Code, failure.Code)
	}
}
