package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/browserops"
	"github.com/pinchtab/pinchtab/internal/browsers"
	_ "github.com/pinchtab/pinchtab/internal/browsers/chrome"
	_ "github.com/pinchtab/pinchtab/internal/browsers/cloak"
	_ "github.com/pinchtab/pinchtab/internal/browsers/ghostchrome"
	"github.com/pinchtab/pinchtab/internal/config"
)

func routeTestConfig() *config.RuntimeConfig {
	return &config.RuntimeConfig{
		ActionTimeout:     time.Second,
		BrowsersAvailable: []string{config.BrowserChrome, config.BrowserCloak, "ghost-chrome"},
	}
}

func assertSingleAttempt(t *testing.T, route *browserops.RouteMetadata, browser string) {
	t.Helper()
	if route == nil {
		t.Fatal("expected route metadata, got nil")
	}
	if len(route.Attempts) != 1 {
		t.Fatalf("len(Attempts) = %d, want 1: %+v", len(route.Attempts), route.Attempts)
	}
	if route.Attempts[0].Browser != browser || !route.Attempts[0].Accepted {
		t.Fatalf("Attempts[0] = %+v, want accepted %q", route.Attempts[0], browser)
	}
	if route.Escalated {
		t.Errorf("Escalated = true, want false on the plain handle path")
	}
	if route.UsedBrowser != browser {
		t.Errorf("UsedBrowser = %q, want %q", route.UsedBrowser, browser)
	}
}

func assertGhostChromeDowngrade(t *testing.T, route *browserops.RouteMetadata) {
	t.Helper()
	if route == nil {
		t.Fatal("expected route metadata, got nil")
	}
	if len(route.Attempts) != 2 {
		t.Fatalf("len(Attempts) = %d, want 2 (ghost-chrome then chrome): %+v", len(route.Attempts), route.Attempts)
	}
	skipped, ran := route.Attempts[0], route.Attempts[1]
	if skipped.Browser != "ghost-chrome" || skipped.Accepted {
		t.Errorf("Attempts[0] = %+v, want ghost-chrome rejected", skipped)
	}
	if skipped.Reason == "" {
		t.Error("Attempts[0].Reason is empty, want the skipping browser's own reason")
	}
	if ran.Browser != config.BrowserChrome || !ran.Accepted {
		t.Errorf("Attempts[1] = %+v, want chrome accepted", ran)
	}
	for _, attempt := range route.Attempts {
		if attempt.Browser == config.BrowserChrome && !attempt.Accepted {
			t.Errorf("chrome recorded as rejected (%+v); chrome is the browser that ran", attempt)
		}
	}
	if !route.Escalated {
		t.Error("Escalated = false, want true on a capability downgrade")
	}
	if route.UsedBrowser != config.BrowserChrome {
		t.Errorf("UsedBrowser = %q, want %q", route.UsedBrowser, config.BrowserChrome)
	}
	if route.RequestedBrowser != "ghost-chrome" {
		t.Errorf("RequestedBrowser = %q, want %q", route.RequestedBrowser, "ghost-chrome")
	}
}

func readRoute(t *testing.T, h *Handlers, browser, recordName, shape string) *browserops.RouteMetadata {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/"+recordName+"?browser="+browser, nil)
	_, route, ok := h.resolveReadRouting(w, r, "", recordName, shape)
	if !ok {
		t.Fatalf("resolveReadRouting(%s) failed: %d %s", recordName, w.Code, w.Body.String())
	}
	return route
}

func TestResolveReadRouting_SingleBrowserRecordsOneAttempt(t *testing.T) {
	reads := []struct {
		name  string
		shape string
	}{
		{"text", browsers.ShapeRenderedRead},
		{"snapshot", browsers.ShapeStaticSnapshot},
		{"capture", browsers.ShapeVisual},
	}

	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			h := New(&mockBridge{}, routeTestConfig(), nil, nil, nil)
			assertSingleAttempt(t, readRoute(t, h, config.BrowserCloak, read.name, read.shape), config.BrowserCloak)
		})
	}
}

func TestResolveReadRouting_GhostChromeDowngradeRecordsBothBrowsers(t *testing.T) {
	h := New(&mockBridge{}, routeTestConfig(), nil, nil, nil)
	assertGhostChromeDowngrade(t, readRoute(t, h, "ghost-chrome", "text", browsers.ShapeRenderedRead))
}

func actionRoute(t *testing.T, browser string) *browserops.RouteMetadata {
	t.Helper()
	h := New(&autoSwitchActionBridge{}, routeTestConfig(), nil, nil, nil)
	body := []byte(`{"kind":"click","browser":"` + browser + `"}`)
	req := httptest.NewRequest("POST", "/action", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Route *browserops.RouteMetadata `json:"route"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Route
}

func TestHandleAction_SingleBrowserRecordsOneAttempt(t *testing.T) {
	assertSingleAttempt(t, actionRoute(t, config.BrowserCloak), config.BrowserCloak)
}

func TestHandleAction_GhostChromeDowngradeRecordsBothBrowsers(t *testing.T) {
	assertGhostChromeDowngrade(t, actionRoute(t, "ghost-chrome"))
}

func multiStepRoute(t *testing.T, path, body string) *browserops.RouteMetadata {
	t.Helper()
	cfg := routeTestConfig()
	cfg.AllowMacro = true
	h := New(&autoSwitchActionBridge{}, cfg, nil, nil, nil)
	req := httptest.NewRequest("POST", path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	if path == "/macro" {
		h.HandleMacro(w, req)
	} else {
		h.HandleActions(w, req)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Route *browserops.RouteMetadata `json:"route"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Route
}

func TestHandleActions_GhostChromeDowngradeRecordsBothBrowsers(t *testing.T) {
	assertGhostChromeDowngrade(t, multiStepRoute(t, "/actions", `{"actions":[{"kind":"click","browser":"ghost-chrome"}]}`))
}

func TestHandleMacro_SingleBrowserRecordsOneAttempt(t *testing.T) {
	assertSingleAttempt(t, multiStepRoute(t, "/macro", `{"steps":[{"kind":"click","browser":"cloak"}]}`), config.BrowserCloak)
}

func TestHandleMacro_GhostChromeDowngradeRecordsBothBrowsers(t *testing.T) {
	assertGhostChromeDowngrade(t, multiStepRoute(t, "/macro", `{"steps":[{"kind":"click","browser":"ghost-chrome"}]}`))
}

func navigateRoute(t *testing.T, browser string) *browserops.RouteMetadata {
	t.Helper()
	stubNavigateHostResolution(t, func(context.Context, string, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	})

	m := &mockBridge{navigateResult: &bridge.NavigateResult{TabID: "tab_abc12345", URL: "https://example.com/"}}
	h := New(m, routeTestConfig(), nil, nil, nil)
	body := []byte(`{"url":"https://example.com","browser":"` + browser + `"}`)
	req := httptest.NewRequest("POST", "/navigate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleNavigate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Route *browserops.RouteMetadata `json:"route"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Route
}

func TestHandleNavigate_SingleBrowserRecordsOneAttempt(t *testing.T) {
	assertSingleAttempt(t, navigateRoute(t, config.BrowserCloak), config.BrowserCloak)
}

func TestHandleNavigate_GhostChromeDowngradeRecordsBothBrowsers(t *testing.T) {
	assertGhostChromeDowngrade(t, navigateRoute(t, "ghost-chrome"))
}
