package mcp

import (
	"strings"
	"testing"
)

func TestHandleNetwork(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network", map[string]any{}, srv)
	text := resultText(t, r)
	if !strings.Contains(text, "/network") {
		t.Errorf("expected /network path, got %s", text)
	}
}

func TestHandleNetworkWithFilters(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network", map[string]any{
		"tabId":  "t1",
		"filter": "api.example",
		"method": "POST",
		"status": "4xx",
		"type":   "xhr",
		"limit":  float64(10),
	}, srv)

	text := resultText(t, r)
	if !strings.Contains(text, "/network") {
		t.Errorf("expected /network path, got %s", text)
	}
	if !strings.Contains(text, "api.example") {
		t.Errorf("expected filter in query, got %s", text)
	}
	if !strings.Contains(text, "POST") {
		t.Errorf("expected method in query, got %s", text)
	}
}

func TestHandleNetworkDetail(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_detail", map[string]any{
		"requestId": "req123",
		"tabId":     "t1",
		"body":      true,
	}, srv)

	text := resultText(t, r)
	if !strings.Contains(text, "/network/req123") {
		t.Errorf("expected /network/req123 path, got %s", text)
	}
}

func TestHandleNetworkDetailMissingRequestId(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_detail", map[string]any{}, srv)
	if !r.IsError {
		t.Error("expected error for missing requestId")
	}
}

func TestHandleNetworkClear(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_clear", map[string]any{}, srv)
	text := resultText(t, r)
	if !strings.Contains(text, "/network/clear") {
		t.Errorf("expected /network/clear path, got %s", text)
	}
}

func TestHandleNetworkClearWithTab(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_clear", map[string]any{
		"tabId": "t1",
	}, srv)

	text := resultText(t, r)
	if !strings.Contains(text, "/network/clear") {
		t.Errorf("expected /network/clear path, got %s", text)
	}
}

func TestHandleNetworkRoute_Abort(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_route", map[string]any{
		"tabId":   "t1",
		"pattern": "*.png",
		"action":  "abort",
	}, srv)

	text := resultText(t, r)
	if !strings.Contains(text, "/tabs/t1/network/route") {
		t.Errorf("expected /tabs/t1/network/route path, got %s", text)
	}
	if !strings.Contains(text, `"action":"abort"`) {
		t.Errorf("expected action=abort in body echo, got %s", text)
	}
}

func TestHandleNetworkRoute_Fulfill_PassesAllFields(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_route", map[string]any{
		"tabId":        "t1",
		"pattern":      "api",
		"action":       "fulfill",
		"body":         `{"k":1}`,
		"contentType":  "application/json",
		"status":       float64(201),
		"resourceType": "xhr",
	}, srv)

	text := resultText(t, r)
	for _, want := range []string{`"contentType":"application/json"`, `"resourceType":"xhr"`, `"status":201`} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %s in body echo, got %s", want, text)
		}
	}
}

func TestHandleNetworkRoute_MissingPattern(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_route", map[string]any{"tabId": "t1"}, srv)
	if !r.IsError {
		t.Error("expected error when pattern missing")
	}
}

func TestHandleNetworkUnroute(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_unroute", map[string]any{
		"tabId":   "t1",
		"pattern": "*.png",
	}, srv)

	text := resultText(t, r)
	if !strings.Contains(text, "/tabs/t1/network/route") {
		t.Errorf("expected /tabs/t1/network/route path, got %s", text)
	}
	if !strings.Contains(text, `"DELETE"`) {
		t.Errorf("expected DELETE method, got %s", text)
	}
	if !strings.Contains(text, "*.png") {
		t.Errorf("expected pattern in query, got %s", text)
	}
}

func TestHandleNetworkUnroute_All(t *testing.T) {
	srv := mockPinchTab()
	defer srv.Close()

	r := callTool(t, "pinchtab_network_unroute", map[string]any{"tabId": "t1"}, srv)
	text := resultText(t, r)
	if !strings.Contains(text, "/tabs/t1/network/route") {
		t.Errorf("expected /tabs/t1/network/route path, got %s", text)
	}
}
