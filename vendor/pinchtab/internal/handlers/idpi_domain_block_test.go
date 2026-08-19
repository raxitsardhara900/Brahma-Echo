package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

func idpiBlockingConfig() *config.RuntimeConfig {
	return &config.RuntimeConfig{
		ActionTimeout:  time.Second,
		AllowCookies:   true,
		AllowedDomains: []string{"example.com"},
		IDPI:           config.IDPIConfig{Enabled: true, StrictMode: true},
	}
}

type blockedResponse struct {
	Code    string         `json:"code"`
	Error   string         `json:"error"`
	Details map[string]any `json:"details"`
}

func decodeBlocked(t *testing.T, w *httptest.ResponseRecorder) blockedResponse {
	t.Helper()
	if w.Code != 403 {
		t.Fatalf("status = %d, want 403: %s", w.Code, w.Body.String())
	}
	var resp blockedResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body %s)", err, w.Body.String())
	}
	if resp.Code != idpiDomainBlockedCode {
		t.Errorf("code = %q, want %q — a consumer must be able to classify every IDPI domain block the same way", resp.Code, idpiDomainBlockedCode)
	}
	return resp
}

func detailString(t *testing.T, details map[string]any, key string) string {
	t.Helper()
	value, ok := details[key].(string)
	if !ok || value == "" {
		t.Fatalf("details.%s missing or empty; details = %v", key, details)
	}
	return value
}

// The reported dead end: every read verb answered 403 with no hint and no remedy,
// and the only actionable string in the message was the allowlist config key —
// the one change that must NOT be made to recover a tab that merely drifted.
func TestDriftedTabBlockOffersBackNotTheAllowlist(t *testing.T) {
	b := &policyMockBridge{
		state: bridge.TabPolicyState{
			CurrentURL: "https://www.iana.org/help/example-domains",
			Blocked:    true,
			Reason:     `domain "www.iana.org" is not in the allowed list (security.allowedDomains)`,
			UpdatedAt:  time.Now(),
		},
		hasState: true,
	}
	h := New(b, idpiBlockingConfig(), nil, nil, nil)

	w := httptest.NewRecorder()
	h.HandleURL(w, httptest.NewRequest("GET", "/url?tabId=tab1", nil))

	resp := decodeBlocked(t, w)
	remedy := detailString(t, resp.Details, "remedy")
	if !strings.Contains(remedy, "pinchtab back") {
		t.Errorf("remedy = %q, want the recovery that actually works (back)", remedy)
	}
	if strings.Contains(remedy, "security.allowedDomains") {
		t.Errorf("remedy = %q must not point a drifted tab at the allowlist; widening it is the change the allowlist exists to prevent", remedy)
	}
	hint := detailString(t, resp.Details, "hint")
	if !strings.Contains(strings.ToLower(hint), "never read") && !strings.Contains(strings.ToLower(hint), "was never") {
		t.Errorf("hint = %q should say the page was never read", hint)
	}
	if got := detailString(t, resp.Details, "domain"); got != "www.iana.org" {
		t.Errorf("details.domain = %q, want the blocked domain as a discrete field", got)
	}
	if got := detailString(t, resp.Details, "url"); got != "https://www.iana.org/help/example-domains" {
		t.Errorf("details.url = %q", got)
	}
}

// enforceURLDomainPolicy is reached only from the cookie endpoints, so it needs
// its own case: a test against /navigate would leave this site untested.
func TestRefusedCookieURLBlockNamesTheAllowlist(t *testing.T) {
	// The current tab must be ON the allowlist, or the drifted-tab check refuses
	// first and this test asserts on the wrong site's remedy.
	b := &policyMockBridge{
		state: bridge.TabPolicyState{
			CurrentURL: "https://example.com/",
			Blocked:    false,
			UpdatedAt:  time.Now(),
		},
		hasState: true,
	}
	h := New(b, idpiBlockingConfig(), nil, nil, nil)

	w := httptest.NewRecorder()
	h.HandleGetCookies(w, httptest.NewRequest("GET", "/cookies?tabId=tab1&url=https://www.iana.org/help", nil))

	resp := decodeBlocked(t, w)
	remedy := detailString(t, resp.Details, "remedy")
	if !strings.Contains(remedy, "security.allowedDomains") {
		t.Errorf("remedy = %q; for a refused URL the allowlist genuinely is the only lever", remedy)
	}
	if strings.Contains(remedy, "pinchtab back") {
		t.Errorf("remedy = %q; nothing navigated, so there is nothing to go back to", remedy)
	}
	if got := detailString(t, resp.Details, "domain"); got != "www.iana.org" {
		t.Errorf("details.domain = %q", got)
	}
}

// The nav site used to answer with the generic "error" code and append the
// allowlist guidance to the MESSAGE. Both are now the shared shape.
func TestBlockedNavigateCarriesTheSharedCodeAndRemedy(t *testing.T) {
	h := New(&policyMockBridge{}, idpiBlockingConfig(), nil, nil, nil)

	w := httptest.NewRecorder()
	h.HandleNavigate(w, httptest.NewRequest("GET", "/navigate?url=https://www.iana.org/help", nil))

	resp := decodeBlocked(t, w)
	if !strings.Contains(resp.Error, "navigation blocked by IDPI") {
		t.Errorf("error = %q, want the navigate-site wording", resp.Error)
	}
	if !strings.Contains(detailString(t, resp.Details, "remedy"), "security.allowedDomains") {
		t.Errorf("details.remedy = %v", resp.Details["remedy"])
	}
}

// A contains-check would pass happily on doubled guidance, which is exactly the
// assertion that would let the duplication ship: the message must no longer carry
// the allowlist instructions now that details.remedy does.
func TestAllowlistGuidanceIsRenderedExactlyOnce(t *testing.T) {
	h := New(&policyMockBridge{}, idpiBlockingConfig(), nil, nil, nil)

	w := httptest.NewRecorder()
	h.HandleNavigate(w, httptest.NewRequest("GET", "/navigate?url=https://www.iana.org/help", nil))
	resp := decodeBlocked(t, w)

	if got := strings.Count(resp.Error, "security.allowedDomains"); got > 1 {
		t.Errorf("the error message names the allowlist %d times: %q", got, resp.Error)
	}
	if strings.Contains(resp.Error, "config set security.allowedDomains") {
		t.Errorf("the message still carries the config-set guidance that details.remedy now renders: %q", resp.Error)
	}

	rendered := resp.Error + "\n" + detailString(t, resp.Details, "hint") + "\n" + detailString(t, resp.Details, "remedy")
	if got := strings.Count(rendered, "config set security.allowedDomains"); got != 1 {
		t.Errorf("the copy-pasteable allowlist command appears %d times in the rendered output, want exactly 1:\n%s", got, rendered)
	}
}

// The two sites must not share one string, which is the whole reason the card
// rejected a single shared remedy.
func TestTheTwoBlockSitesGiveDifferentRemedies(t *testing.T) {
	drifted := idpiDriftedTabDetails("https://www.iana.org/help")
	refused := idpiRefusedURLDetails("https://www.iana.org/help")

	if drifted["remedy"] == refused["remedy"] {
		t.Fatalf("both sites render the same remedy %q; a drifted tab recovers with back, a refused URL does not", drifted["remedy"])
	}
	if drifted["hint"] == refused["hint"] {
		t.Errorf("both sites render the same hint %q", drifted["hint"])
	}
}
