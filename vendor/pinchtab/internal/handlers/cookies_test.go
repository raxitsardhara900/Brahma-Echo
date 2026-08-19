package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

func TestHandleSetCookies_InvalidJSON(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	req := httptest.NewRequest("POST", "/cookies", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()

	h.HandleSetCookies(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetCookies_DisabledByDefault(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/cookies", nil)
	w := httptest.NewRecorder()

	h.HandleGetCookies(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("security.allowCookies")) {
		t.Fatalf("expected allowCookies hint, got %s", w.Body.String())
	}
}

func TestHandleSetCookies_DisabledByDefault(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("POST", "/cookies", bytes.NewReader([]byte(`{"url":"https://pinchtab.com","cookies":[{"name":"a","value":"b"}]}`)))
	w := httptest.NewRecorder()

	h.HandleSetCookies(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("security.allowCookies")) {
		t.Fatalf("expected allowCookies hint, got %s", w.Body.String())
	}
}

func TestHandleClearCookies_DisabledByDefault(t *testing.T) {
	b := &cookieClearMockBridge{}
	h := New(b, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("DELETE", "/cookies", nil)
	w := httptest.NewRecorder()

	h.HandleClearCookies(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if b.clearCookiesCalled {
		t.Fatal("expected ClearCookies not to be called")
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("security.allowCookies")) {
		t.Fatalf("expected allowCookies hint, got %s", w.Body.String())
	}
}

func TestHandleSetCookies_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	body := `{"url":"https://pinchtab.com","cookies":[{"name":"a","value":"b"}],"tabId":"nonexistent"}`
	req := httptest.NewRequest("POST", "/cookies", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	h.HandleSetCookies(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetCookies_NameFilter(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/cookies?name=session_id&tabId=nonexistent", nil)
	w := httptest.NewRecorder()

	h.HandleGetCookies(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["error"] == nil {
		t.Error("expected error in response")
	}
}

func TestHandleTabGetCookies_MissingTabID(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/tabs//cookies", nil)
	w := httptest.NewRecorder()
	h.HandleTabGetCookies(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTabGetCookies_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/tabs/tab_abc/cookies", nil)
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabGetCookies(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleTabSetCookies_TabIDMismatch(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	body := `{"tabId":"tab_other","url":"https://pinchtab.com","cookies":[{"name":"a","value":"b"}]}`
	req := httptest.NewRequest("POST", "/tabs/tab_abc/cookies", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetCookies(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTabSetCookies_NoTab(t *testing.T) {
	h := New(&failMockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	body := `{"url":"https://pinchtab.com","cookies":[{"name":"a","value":"b"}]}`
	req := httptest.NewRequest("POST", "/tabs/tab_abc/cookies", bytes.NewReader([]byte(body)))
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabSetCookies(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// cookieClearMockBridge provides BrowserContext and ClearCookies for handler tests.
type cookieClearMockBridge struct {
	mockBridge
	clearCookiesCalled bool
	clearCookiesErr    error
}

func (m *cookieClearMockBridge) BrowserContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func (m *cookieClearMockBridge) ClearCookies(ctx context.Context) error {
	m.clearCookiesCalled = true
	return m.clearCookiesErr
}

func TestHandleClearCookies_Success(t *testing.T) {
	b := &cookieClearMockBridge{}
	h := New(b, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/cookies", nil)
	w := httptest.NewRecorder()
	h.HandleClearCookies(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !b.clearCookiesCalled {
		t.Fatal("expected ClearCookies to be called")
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "cleared" {
		t.Errorf("expected status=cleared, got %v", resp["status"])
	}
}

func TestHandleTabClearCookies_Success(t *testing.T) {
	b := &cookieClearMockBridge{}
	h := New(b, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/tabs/tab1/cookies", nil)
	req.SetPathValue("id", "tab1")
	w := httptest.NewRecorder()
	h.HandleTabClearCookies(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !b.clearCookiesCalled {
		t.Fatal("expected ClearCookies to be called")
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "cleared" {
		t.Errorf("expected status=cleared, got %v", resp["status"])
	}
}

func TestHandleTabClearCookies_MissingTabID(t *testing.T) {
	h := New(&cookieClearMockBridge{}, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/tabs//cookies", nil)
	w := httptest.NewRecorder()
	h.HandleTabClearCookies(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTabClearCookies_DisabledByDefault(t *testing.T) {
	b := &cookieClearMockBridge{}
	h := New(b, &config.RuntimeConfig{}, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/tabs/tab1/cookies", nil)
	req.SetPathValue("id", "tab1")
	w := httptest.NewRecorder()
	h.HandleTabClearCookies(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if b.clearCookiesCalled {
		t.Fatal("expected ClearCookies not to be called")
	}
}

func TestHandleTabClearCookies_NoTab(t *testing.T) {
	b := &cookieClearMockBridge{}
	b.failTab = true
	h := New(b, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)

	req := httptest.NewRequest("DELETE", "/tabs/nonexistent/cookies", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.HandleTabClearCookies(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleClearCookies_RouteRegistration(t *testing.T) {
	b := &cookieClearMockBridge{}
	h := New(b, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, nil)

	req := httptest.NewRequest("DELETE", "/cookies", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected DELETE /cookies to be registered, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("DELETE", "/tabs/tab1/cookies", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected DELETE /tabs/{id}/cookies to be registered, got %d: %s", w.Code, w.Body.String())
	}
}

// cookieJarBridge records what reached the browser and gives the tab a current URL,
// which is what the set path defaults to.
type cookieJarBridge struct {
	mockBridge
	currentURL string
	setErr     error
	set        []bridge.SetCookieParams
}

func (b *cookieJarBridge) CurrentURL(context.Context) (string, error) { return b.currentURL, nil }

func (b *cookieJarBridge) SetCookie(_ context.Context, params bridge.SetCookieParams) error {
	if b.setErr != nil {
		return b.setErr
	}
	b.set = append(b.set, params)
	return nil
}

func postCookies(t *testing.T, b *cookieJarBridge, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := New(b, &config.RuntimeConfig{AllowCookies: true}, nil, nil, nil)
	w := httptest.NewRecorder()
	h.HandleSetCookies(w, httptest.NewRequest("POST", "/cookies", bytes.NewReader([]byte(body))))
	return w
}

// An empty value blanks a cookie without deleting it, which is a real operation. It
// used to be skipped by the loop, so the browser was never asked and the caller was
// told nothing useful.
func TestHandleSetCookiesHonoursAnEmptyValueAndDefaultsTheURLToTheTab(t *testing.T) {
	b := &cookieJarBridge{currentURL: "http://example.com/app"}
	w := postCookies(t, b, `{"cookies":[{"name":"blank","value":""}]}`)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["set"] != float64(1) || resp["failed"] != float64(0) {
		t.Errorf("response = %+v, want set 1 failed 0 — a cookie the browser stored must count as set", resp)
	}
	if len(b.set) != 1 {
		t.Fatalf("browser saw %d cookies, want the one with the empty value", len(b.set))
	}
	if b.set[0].Value != "" {
		t.Errorf("value = %q, want it kept empty", b.set[0].Value)
	}
	if b.set[0].URL != "http://example.com/app" {
		t.Errorf("url = %q, want the tab's current URL; a caller driving one tab must not have to restate it", b.set[0].URL)
	}
}

// A nameless cookie cannot be set at all, so it is refused rather than skipped: a
// skipped cookie is the silent no-op this endpoint used to answer with.
func TestHandleSetCookiesRefusesANamelessCookieInsteadOfSkippingIt(t *testing.T) {
	b := &cookieJarBridge{currentURL: "http://example.com/app"}
	w := postCookies(t, b, `{"cookies":[{"name":"","value":"x"}]}`)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	if len(b.set) != 0 {
		t.Errorf("browser was asked to store %+v; a nameless cookie cannot be set", b.set)
	}
}

func TestHandleSetCookiesCountsARejectedCookieAsFailed(t *testing.T) {
	b := &cookieJarBridge{currentURL: "http://example.com/app", setErr: fmt.Errorf("cdp refused")}
	w := postCookies(t, b, `{"cookies":[{"name":"a","value":"b"}]}`)

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["set"] != float64(0) || resp["failed"] != float64(1) {
		t.Errorf("response = %+v, want set 0 failed 1", resp)
	}
}

// The default has to come from somewhere: with no current URL the request is refused,
// rather than sending CDP a cookie with no target.
func TestHandleSetCookiesRefusesWhenTheTabHasNoURLToDefaultTo(t *testing.T) {
	b := &cookieJarBridge{}
	w := postCookies(t, b, `{"cookies":[{"name":"a","value":"b"}]}`)

	if w.Code != 400 || !strings.Contains(w.Body.String(), "url is required") {
		t.Fatalf("status = %d body = %s, want a 400 naming the missing url", w.Code, w.Body.String())
	}
	if len(b.set) != 0 {
		t.Errorf("browser was asked to store %+v with no URL", b.set)
	}
}

// enforceURLDomainPolicy has exactly two production call sites, both in cookies.go, and
// only the GET one was pinned: idpi_domain_block_test.go's
// TestRefusedCookieURLBlockNamesTheAllowlist drives HandleGetCookies with a refused URL,
// but its subject is the REMEDY WORDING — it pins that call site only incidentally,
// because a missing block leaves no blocked response to decode. The three tests below own
// the POST site and assert ENFORCEMENT instead: the cookie is not stored. Neither test
// covers the other's site, so neither may be deleted believing it does.
type policyCookieBridge struct {
	cookieJarBridge
	tabState bridge.TabPolicyState
}

func (b *policyCookieBridge) GetTabPolicyState(string) (bridge.TabPolicyState, bool) {
	return b.tabState, true
}

// The cached tab state is ALLOWED and fresh on purpose: the drifted-tab check runs before
// the URL policy, and a refused tab would refuse first — leaving these tests green with
// the URL policy deleted, which is the trap this whole card exists to close.
func newPolicyCookieBridge(currentURL string) *policyCookieBridge {
	b := &policyCookieBridge{tabState: bridge.TabPolicyState{
		CurrentURL: "https://example.com/app",
		UpdatedAt:  time.Now(),
	}}
	b.currentURL = currentURL
	return b
}

func postCookiesUnderPolicy(t *testing.T, b *policyCookieBridge, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := New(b, idpiBlockingConfig(), nil, nil, nil)
	w := httptest.NewRecorder()
	h.HandleSetCookies(w, httptest.NewRequest("POST", "/cookies", bytes.NewReader([]byte(body))))
	return w
}

func TestSetCookiesRefusesAURLTheDomainPolicyBlocks(t *testing.T) {
	b := newPolicyCookieBridge("https://example.com/app")
	w := postCookiesUnderPolicy(t, b, `{"url":"https://www.iana.org/help","cookies":[{"name":"session","value":"abc"}]}`)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a URL outside the allowlist: %s", w.Code, w.Body.String())
	}
	if len(b.set) != 0 {
		t.Errorf("the browser was asked to store %+v for a domain this instance may not touch", b.set)
	}
}

// The value the policy checks can arrive implicitly: with url absent it is the tab's
// current URL, so the check has to run on the RESOLVED value. Moving it above the
// defaulting step hands the policy an empty string, which it allows, and the cookie is
// written for whatever domain the tab happens to be on.
func TestSetCookiesEnforcesTheDomainPolicyOnTheDefaultedURL(t *testing.T) {
	b := newPolicyCookieBridge("https://www.iana.org/help")
	w := postCookiesUnderPolicy(t, b, `{"cookies":[{"name":"session","value":"abc"}]}`)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; the defaulted URL is outside the allowlist: %s", w.Code, w.Body.String())
	}
	if len(b.set) != 0 {
		t.Errorf("the browser was asked to store %+v for the refused domain the tab defaulted to", b.set)
	}
}

// The other half of the pair: a guard that refuses everything would satisfy both tests
// above, and would break cookie injection entirely.
func TestSetCookiesStoresACookieForAnAllowedURL(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{name: "url stated", body: `{"url":"https://example.com/app","cookies":[{"name":"session","value":"abc"}]}`},
		{name: "url defaulted from the tab", body: `{"cookies":[{"name":"session","value":"abc"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newPolicyCookieBridge("https://example.com/app")
			w := postCookiesUnderPolicy(t, b, tc.body)

			if w.Code != 200 {
				t.Fatalf("status = %d, want 200 for an allowed URL: %s", w.Code, w.Body.String())
			}
			if len(b.set) != 1 {
				t.Fatalf("browser saw %d cookies, want the one that was allowed", len(b.set))
			}
			if b.set[0].URL != "https://example.com/app" {
				t.Errorf("stored against %q, want the allowed URL", b.set[0].URL)
			}
		})
	}
}
