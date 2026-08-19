package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

// refusingCookieBridge stores nothing and says why, which is what CDP does for a domain
// mismatch, an invalid SameSite or Secure on a plain-http URL.
type refusingCookieBridge struct {
	*mockBridge
	reason string
}

func (b *refusingCookieBridge) SetCookie(context.Context, bridge.SetCookieParams) error {
	return errCookieRefused{b.reason}
}

func (b *refusingCookieBridge) CurrentURL(context.Context) (string, error) {
	return "https://example.com/", nil
}

type errCookieRefused struct{ reason string }

func (e errCookieRefused) Error() string { return e.reason }

// The counts alone are not actionable: a domain mismatch, an invalid SameSite and
// Secure-on-http are all "failed: 1" and want three different corrections. A caller that
// learns only the count cannot tell a malformed request from a retryable one, and both the
// CLI and the MCP tool can only report what the endpoint says.
func TestHandleSetCookies_ReportsWhyACookieWasRefused(t *testing.T) {
	h := New(&refusingCookieBridge{mockBridge: &mockBridge{}, reason: "invalid sameSite value"},
		&config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)

	body := `{"cookies":[{"name":"sid","value":"abc","sameSite":"Sometimes"}]}`
	req := httptest.NewRequest("POST", "/cookies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSetCookies(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var got struct {
		Set      int `json:"set"`
		Failed   int `json:"failed"`
		Total    int `json:"total"`
		Failures []struct {
			Index int    `json:"index"`
			Name  string `json:"name"`
			Error string `json:"error"`
		} `json:"failures"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got.Set != 0 || got.Failed != 1 || got.Total != 1 {
		t.Errorf("counts = set %d failed %d total %d, want 0/1/1", got.Set, got.Failed, got.Total)
	}
	if len(got.Failures) != 1 {
		t.Fatalf("failures = %+v, want one entry naming the refused cookie", got.Failures)
	}
	if got.Failures[0].Name != "sid" || got.Failures[0].Index != 0 {
		t.Errorf("failure = %+v, want index 0 and name sid so a multi-cookie request says which", got.Failures[0])
	}
	if !strings.Contains(got.Failures[0].Error, "invalid sameSite value") {
		t.Errorf("failure error = %q, want the reason the browser gave", got.Failures[0].Error)
	}
}

// A request where everything landed must not carry an empty failures array — a caller
// checking for the key's presence would read that as a partial failure.
func TestHandleSetCookies_OmitsFailuresWhenEveryCookieLands(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)

	body := `{"url":"https://example.com/","cookies":[{"name":"sid","value":"abc"}]}`
	req := httptest.NewRequest("POST", "/cookies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSetCookies(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, present := got["failures"]; present {
		t.Errorf("response carries a failures key with nothing wrong: %s", w.Body.String())
	}
}
