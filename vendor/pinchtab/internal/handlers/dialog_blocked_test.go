package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

type dialogBlockedFailure struct {
	Code      string `json:"code"`
	Error     string `json:"error"`
	Retryable bool   `json:"retryable"`
	Details   struct {
		TabID         string `json:"tabId"`
		DialogType    string `json:"dialogType"`
		DialogMessage string `json:"dialogMessage"`
		Hint          string `json:"hint"`
		Remedy        string `json:"remedy"`
	} `json:"details"`
}

func decodeDialogBlocked(t *testing.T, w *httptest.ResponseRecorder, dialog *bridge.DialogState) {
	t.Helper()
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	var failure dialogBlockedFailure
	if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode failure: %v body=%s", err, w.Body.String())
	}
	if failure.Code != dialogBlockedCode {
		t.Fatalf("code = %q, want %q; body=%s", failure.Code, dialogBlockedCode, w.Body.String())
	}
	if failure.Retryable {
		t.Fatalf("dialog-blocked response advertises retryable: %s", w.Body.String())
	}
	if failure.Details.DialogType != dialog.Type || failure.Details.DialogMessage != dialog.Message {
		t.Fatalf("dialog not named: %s", w.Body.String())
	}
	if failure.Details.Remedy != dialogBlockedRemedy.String() {
		t.Fatalf("remedy = %q, want %q", failure.Details.Remedy, dialogBlockedRemedy)
	}
	if failure.Details.Hint == "" {
		t.Fatalf("missing hint: %s", w.Body.String())
	}
}

func TestDialogBlockedIsTheSameContractOnReadsAndActions(t *testing.T) {
	dialogs := []*bridge.DialogState{
		{Type: "alert", Message: "hello alert"},
		{Type: "confirm", Message: "are you sure?"},
		{Type: "prompt", Message: "your name?", DefaultPrompt: "anon"},
	}

	paths := []struct {
		name string
		call func(h *Handlers, w *httptest.ResponseRecorder)
	}{
		{"url", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleURL(w, httptest.NewRequest("GET", "/url?tabId=tab1", nil))
		}},
		{"title", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleTitle(w, httptest.NewRequest("GET", "/title?tabId=tab1", nil))
		}},
		{"text", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleText(w, httptest.NewRequest("GET", "/text?tabId=tab1", nil))
		}},
		{"snapshot", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleSnapshot(w, httptest.NewRequest("GET", "/snapshot?tabId=tab1", nil))
		}},
		{"action", func(h *Handlers, w *httptest.ResponseRecorder) {
			body := bytes.NewReader([]byte(`{"kind":"click","tabId":"tab1","nodeId":42}`))
			h.HandleAction(w, httptest.NewRequest("POST", "/action", body))
		}},
		// The visual exports ride resolveBinaryReadContext, the sibling prelude —
		// these rows are what stop the two preludes diverging on the guard again.
		{"screenshot", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleScreenshot(w, httptest.NewRequest("GET", "/screenshot?tabId=tab1", nil))
		}},
		{"pdf", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandlePDF(w, httptest.NewRequest("GET", "/pdf?tabId=tab1", nil))
		}},
		{"annotate", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleAnnotate(w, httptest.NewRequest("GET", "/annotate?tabId=tab1", nil))
		}},
	}

	for _, dialog := range dialogs {
		for _, path := range paths {
			t.Run(dialog.Type+"/"+path.name, func(t *testing.T) {
				mb := &mockBridge{}
				mb.GetDialogManager().SetPending("tab1", dialog)
				h := New(mb, &config.RuntimeConfig{ActionTimeout: time.Second}, nil, nil, nil)

				w := httptest.NewRecorder()
				path.call(h, w)
				decodeDialogBlocked(t, w, dialog)
			})
		}
	}
}

func TestBusyTabWithAPendingDialogIsDialogBlockedNotUnresponsive(t *testing.T) {
	dialog := &bridge.DialogState{Type: "alert", Message: "hello alert"}
	b := &busyPolicyBridge{currentURLErr: context.DeadlineExceeded}
	b.GetDialogManager().SetPending("exact-tab", dialog)
	h := New(b, &config.RuntimeConfig{
		AllowedDomains: []string{"example.com"},
		IDPI:           config.IDPIConfig{Enabled: true, StrictMode: true},
	}, nil, nil, nil)

	w := httptest.NewRecorder()
	h.handleWaitCore(w, httptest.NewRequest("POST", "/tabs/exact-tab/wait", nil), waitRequest{TabID: "exact-tab", Text: "ready"})

	decodeDialogBlocked(t, w, dialog)
	if strings.Contains(w.Body.String(), "fresh tab") {
		t.Fatalf("dialog-blocked response suggests a fresh tab: %s", w.Body.String())
	}
}

func TestReadsAreNotDialogBlockedWithoutAPendingDialog(t *testing.T) {
	for _, path := range []struct {
		name string
		call func(h *Handlers, w *httptest.ResponseRecorder)
	}{
		{"url", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleURL(w, httptest.NewRequest("GET", "/url?tabId=tab1", nil))
		}},
		{"screenshot", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleScreenshot(w, httptest.NewRequest("GET", "/screenshot?tabId=tab1", nil))
		}},
		{"pdf", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandlePDF(w, httptest.NewRequest("GET", "/pdf?tabId=tab1", nil))
		}},
		{"annotate", func(h *Handlers, w *httptest.ResponseRecorder) {
			h.HandleAnnotate(w, httptest.NewRequest("GET", "/annotate?tabId=tab1", nil))
		}},
	} {
		t.Run(path.name, func(t *testing.T) {
			mb := &mockBridge{}
			h := New(mb, &config.RuntimeConfig{ActionTimeout: time.Second}, nil, nil, nil)

			w := httptest.NewRecorder()
			path.call(h, w)
			if w.Code == http.StatusConflict {
				t.Fatalf("refused as dialog-blocked with no dialog pending: %s", w.Body.String())
			}
		})
	}
}
