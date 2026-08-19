package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// Error details carry page-controlled data — dialog text, document titles,
// URLs — and the CLI prints some of them straight to a terminal. They need the
// same cleaning the top-level message already gets.
func TestErrorCodeSanitizesDetailStrings(t *testing.T) {
	rec := httptest.NewRecorder()
	details := map[string]any{
		"title":     "Login\x1b[31m failed\x00",
		"count":     3,
		"retryable": true,
	}
	ErrorCode(rec, 400, "bad_input", "boom", false, details)

	body := rec.Body.String()
	if strings.Contains(body, "\\u001b") || strings.Contains(body, "\x1b") {
		t.Errorf("ANSI escape survived into details: %s", body)
	}
	if strings.Contains(body, "\\u0000") {
		t.Errorf("null byte survived into details: %s", body)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	got, _ := payload["details"].(map[string]any)
	if got["count"] != float64(3) || got["retryable"] != true {
		t.Errorf("non-string detail values must pass through unchanged, got %+v", got)
	}

	// The caller's map must not be rewritten underneath it.
	if details["title"] != "Login\x1b[31m failed\x00" {
		t.Errorf("caller's details map was mutated: %q", details["title"])
	}
}

// An intentionally empty detail must stay empty rather than becoming the
// message-level "error" placeholder.
func TestErrorCodeKeepsEmptyDetailStringsEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	ErrorCode(rec, 400, "bad_input", "boom", false, map[string]any{"hint": ""})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	got, _ := payload["details"].(map[string]any)
	if hint, ok := got["hint"].(string); !ok || hint != "" {
		t.Errorf("empty hint = %q, want empty string", hint)
	}
}

func TestProblemSanitizesDetailStrings(t *testing.T) {
	rec := httptest.NewRecorder()
	Problem(rec, 502, "backend_unavailable", "boom", true, map[string]any{
		"target": "http://host\x1b[31m/path",
	})

	if body := rec.Body.String(); strings.Contains(body, "\\u001b") || strings.Contains(body, "\x1b") {
		t.Errorf("ANSI escape survived into problem details: %s", body)
	}
}
