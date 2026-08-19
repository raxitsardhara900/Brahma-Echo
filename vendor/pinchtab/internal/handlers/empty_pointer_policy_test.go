package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/activity"
)

func TestIsNoCurrentTab(t *testing.T) {
	if !IsNoCurrentTab(ErrNoCurrentTab) {
		t.Fatal("ErrNoCurrentTab itself should match")
	}
	wrapped := fmt.Errorf("ctx: %w", ErrNoCurrentTab)
	if !IsNoCurrentTab(wrapped) {
		t.Fatal("wrapped ErrNoCurrentTab should match")
	}
	if IsNoCurrentTab(errors.New("unrelated")) {
		t.Fatal("unrelated error should not match")
	}
	if IsNoCurrentTab(nil) {
		t.Fatal("nil should not match")
	}
}

func TestWriteTabContextError_NoCurrentTabIs409(t *testing.T) {
	w := httptest.NewRecorder()
	WriteTabContextError(w, noCurrentTabError(`session "ses_1"`), 0)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	if got := w.Body.String(); got == "" || !bytes.Contains([]byte(got), []byte(`"no_current_tab"`)) {
		t.Fatalf("body should include code no_current_tab, got %q", got)
	}
}

func TestWriteTabContextError_OtherErrorsUseFallbackStatus(t *testing.T) {
	w := httptest.NewRecorder()
	WriteTabContextError(w, errors.New("tab not found"), http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestWriteTabContextError_DefaultsTo404WhenZero(t *testing.T) {
	w := httptest.NewRecorder()
	WriteTabContextError(w, errors.New("oops"), 0)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestWriteTabContextError_NilNoOp(t *testing.T) {
	w := httptest.NewRecorder()
	WriteTabContextError(w, nil, 0)
	if w.Code != http.StatusOK {
		t.Fatalf("nil err should not write a response; got status %d", w.Code)
	}
}

func TestTabContext_NoCurrentTabReturnsSentinel(t *testing.T) {
	h, _ := newScopedCurrentTabHandler()

	req := trustedSessionRequest("GET", "/text", "ses_orphan", nil)
	_, _, err := h.tabContext(req, "")
	if !IsNoCurrentTab(err) {
		t.Fatalf("expected ErrNoCurrentTab, got %v", err)
	}
}

func TestTabContext_StalePointerReturnsSentinelAndClears(t *testing.T) {
	h, _ := newScopedCurrentTabHandler()

	// Seed a pointer to a tab the bridge mock does not know about.
	req := trustedSessionRequest("GET", "/text", "ses_2", nil)
	h.CurrentTabs.Set(currentTabScopeFromRequest(req), "tab-does-not-exist")

	_, _, err := h.tabContext(req, "")
	if !IsNoCurrentTab(err) {
		t.Fatalf("stale pointer should produce ErrNoCurrentTab, got %v", err)
	}
	// Pointer should have been cleared.
	if got, ok := h.CurrentTabs.Get(currentTabScopeFromRequest(req)); ok {
		t.Fatalf("pointer should be cleared, still %q", got)
	}
}

func TestNavigateStrict_RejectsIdentifiedCallerWithoutScopedTab(t *testing.T) {
	h, b := newScopedCurrentTabHandler()
	h.SetEmptyPointerPolicy(EmptyPointerStrict)

	req := httptest.NewRequest("POST", "/navigate", bytes.NewReader([]byte(`{"url":"about:blank"}`)))
	req.Header.Set(activity.HeaderAgentID, "agent-strict")

	w := httptest.NewRecorder()
	h.HandleNavigate(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"no_current_tab"`)) {
		t.Fatalf("body should include code no_current_tab, got %s", w.Body.String())
	}
	if len(b.created) != 0 {
		t.Fatalf("strict mode must not create a tab, created %v", b.created)
	}
}

func TestNavigateStrict_PassesAnonymousCallersThrough(t *testing.T) {
	h, b := newScopedCurrentTabHandler()
	h.SetEmptyPointerPolicy(EmptyPointerStrict)

	// Anonymous (no agent / session) — strict policy applies only to
	// identified callers; the bridge mock cannot complete a real navigate
	// so we ignore the eventual error and just check CreateTab was called.
	req := httptest.NewRequest("POST", "/navigate", bytes.NewReader([]byte(`{"url":"about:blank"}`)))

	w := httptest.NewRecorder()
	h.HandleNavigate(w, req)

	if w.Code == http.StatusConflict {
		t.Fatalf("anonymous request must not be rejected by strict policy; got 409 %s", w.Body.String())
	}
	if len(b.created) != 1 {
		t.Fatalf("anonymous strict should still create a tab, created %v", b.created)
	}
}

func TestNavigateLazy_IdentifiedCallerStillCreatesTab(t *testing.T) {
	h, b := newScopedCurrentTabHandler()
	if h.EmptyPointerPolicy() != EmptyPointerLazy {
		t.Fatalf("default policy = %q, want lazy", h.EmptyPointerPolicy())
	}

	req := httptest.NewRequest("POST", "/navigate", bytes.NewReader([]byte(`{"url":"about:blank"}`)))
	req.Header.Set(activity.HeaderAgentID, "agent-lazy")

	w := httptest.NewRecorder()
	h.HandleNavigate(w, req)

	if w.Code == http.StatusConflict {
		t.Fatalf("lazy mode must not return 409; got %s", w.Body.String())
	}
	if len(b.created) != 1 {
		t.Fatalf("lazy identified should create a tab, created %v", b.created)
	}
}
