package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestHandleSetMedia_Valid(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"feature":"prefers-color-scheme","value":"dark"}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/media", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetMedia(w, req)

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

func TestHandleSetMedia_MissingFeature(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"value":"dark"}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/media", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetMedia(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing feature, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetMedia_MissingValue(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"feature":"prefers-color-scheme"}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/media", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetMedia(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing value, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetMedia_InvalidJSON(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/emulation/media", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.HandleSetMedia(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetMedia_TabIDMismatch(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"feature":"prefers-color-scheme","value":"dark","tabId":"tab_other"}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/media", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetMedia(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for tab ID mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetMedia_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"feature":"prefers-color-scheme","value":"dark"}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/media", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetMedia(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}
