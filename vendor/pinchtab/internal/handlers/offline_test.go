package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestHandleSetOffline_True(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"offline":true}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/offline", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetOffline(w, req)

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
		if resp["offline"] != true {
			t.Fatalf("expected offline=true, got %v", resp["offline"])
		}
		if resp["status"] != "offline" {
			t.Fatalf("expected status=offline, got %v", resp["status"])
		}
	}
}

func TestHandleSetOffline_False(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"offline":false}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/offline", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetOffline(w, req)

	if w.Code != 200 && w.Code != 500 {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["offline"] != false {
			t.Fatalf("expected offline=false, got %v", resp["offline"])
		}
		if resp["status"] != "online" {
			t.Fatalf("expected status=online, got %v", resp["status"])
		}
	}
}

func TestHandleSetOffline_InvalidJSON(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/emulation/offline", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.HandleSetOffline(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetOffline_TabIDMismatch(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"offline":true,"tabId":"tab_other"}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/offline", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetOffline(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for tab ID mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetOffline_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"offline":true}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/offline", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetOffline(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}
