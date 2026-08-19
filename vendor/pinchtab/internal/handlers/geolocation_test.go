package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestHandleSetGeolocation_ValidCoords(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":37.7749,"longitude":-122.4194}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	// The mock bridge returns a cancelled context, so chromedp.Run will fail
	// with a context error. We accept 200 or 500 (CDP failure on mock).
	if w.Code != 200 && w.Code != 500 {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_LatitudeTooHigh(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":91,"longitude":0}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for latitude > 90, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_LatitudeTooLow(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":-91,"longitude":0}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for latitude < -90, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_LongitudeTooHigh(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":0,"longitude":181}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for longitude > 180, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_LongitudeTooLow(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":0,"longitude":-181}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for longitude < -180, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_DefaultAccuracy(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":37.7749,"longitude":-122.4194}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	// Even if chromedp fails (mock context), verify the response contains
	// default accuracy when the request succeeds (200) or the validation passed.
	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if acc, ok := resp["accuracy"].(float64); !ok || acc != 1.0 {
			t.Fatalf("expected accuracy=1.0, got %v", resp["accuracy"])
		}
	}
}

func TestHandleSetGeolocation_CustomAccuracy(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":37.7749,"longitude":-122.4194,"accuracy":50}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code == 200 {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if acc, ok := resp["accuracy"].(float64); !ok || acc != 50.0 {
			t.Fatalf("expected accuracy=50.0, got %v", resp["accuracy"])
		}
	}
}

func TestHandleSetGeolocation_InvalidJSON(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_NoTab(t *testing.T) {
	h := New(&mockBridge{failTab: true}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":37.7749,"longitude":-122.4194,"tabId":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetGeolocation_TabIDMismatch(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":37.7749,"longitude":-122.4194,"tabId":"tab_other"}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/geolocation", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for tab ID mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleTabSetGeolocation_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":37.7749,"longitude":-122.4194}`
	req := httptest.NewRequest(http.MethodPost, "/tabs/tab_abc/emulation/geolocation", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetGeolocation(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for nonexistent tab, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetGeolocation_NegativeAccuracy(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)

	body := `{"latitude":0,"longitude":0,"accuracy":-1}`
	req := httptest.NewRequest(http.MethodPost, "/emulation/geolocation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleSetGeolocation(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for negative accuracy, got %d: %s", w.Code, w.Body.String())
	}
}
