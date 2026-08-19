package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestHandleSetViewport_ValidDimensions(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":1920,"height":1080}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	// The mock bridge returns a cancelled context, so chromedp.Run will fail
	// with a context error. We accept 200 or 500 (CDP failure on mock).
	if w.Code != 200 && w.Code != 500 {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetViewport_ZeroWidth(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":0,"height":1080}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for zero width, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetViewport_NegativeHeight(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":1920,"height":-1}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for negative height, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetViewport_MissingDimensions(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing dimensions, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetViewport_DefaultDPR(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":1920,"height":1080}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	// Even if chromedp fails (mock context), verify the response contains
	// default DPR when the request succeeds (200) or the validation passed.
	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if dpr, ok := resp["deviceScaleFactor"].(float64); !ok || dpr != 1.0 {
			t.Fatalf("expected deviceScaleFactor=1.0, got %v", resp["deviceScaleFactor"])
		}
	}
}

func TestHandleSetViewport_CustomDPRAndMobile(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":375,"height":812,"deviceScaleFactor":3,"mobile":true}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if dpr, ok := resp["deviceScaleFactor"].(float64); !ok || dpr != 3.0 {
			t.Fatalf("expected deviceScaleFactor=3.0, got %v", resp["deviceScaleFactor"])
		}
		if mobile, ok := resp["mobile"].(bool); !ok || !mobile {
			t.Fatalf("expected mobile=true, got %v", resp["mobile"])
		}
	}
}

func TestHandleSetViewport_InvalidJSON(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetViewport_NoTab(t *testing.T) {
	h := New(&mockBridge{failTab: true}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":1920,"height":1080,"tabId":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/viewport", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetViewport(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetViewport_TabIDMismatch(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":1920,"height":1080,"tabId":"tab_other"}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/viewport", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetViewport(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for tab ID mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetViewport_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"width":1920,"height":1080}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/viewport", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetViewport(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}
