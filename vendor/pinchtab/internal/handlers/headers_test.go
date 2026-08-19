package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestHandleSetHeaders_Single(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"headers":{"X-Auth-Token":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/headers", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetHeaders(w, req)

	// The mock bridge returns a cancelled context, so chromedp.Run will fail
	// with a context error. We accept 200 or 500 (CDP failure on mock).
	if w.Code != 200 && w.Code != 500 {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["status"] != "applied" {
			t.Fatalf("expected status=applied, got %v", resp["status"])
		}
	}
}

func TestHandleSetHeaders_Multiple(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"headers":{"X-Auth-Token":"abc123","X-Request-ID":"req-456"}}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/headers", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetHeaders(w, req)

	if w.Code != 200 && w.Code != 500 {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["status"] != "applied" {
			t.Fatalf("expected status=applied, got %v", resp["status"])
		}
	}
}

func TestHandleSetHeaders_EmptyMap(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"headers":{}}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/headers", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetHeaders(w, req)

	// Empty headers map is valid — clears extra headers.
	if w.Code != 200 && w.Code != 500 {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetHeaders_InvalidJSON(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/emulation/headers", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.HandleSetHeaders(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetHeaders_MissingHeaders(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"tabId":"tab_123"}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/headers", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetHeaders(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing headers, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetHeaders_TabIDMismatch(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"headers":{"X-Auth":"tok"},"tabId":"tab_other"}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/headers", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetHeaders(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for tab ID mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetHeaders_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"headers":{"X-Auth":"tok"}}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/headers", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetHeaders(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}
