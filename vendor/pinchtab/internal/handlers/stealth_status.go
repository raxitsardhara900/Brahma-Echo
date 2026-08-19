package handlers

import (
	"net/http"

	"github.com/pinchtab/pinchtab/internal/httpx"
	"github.com/pinchtab/pinchtab/internal/stealth"
)

func (h *Handlers) HandleStealthStatus(w http.ResponseWriter, r *http.Request) {
	status := h.Bridge.StealthStatus()
	if status == nil {
		writeUnavailable(w, 503, "stealth_unavailable", "stealth bundle unavailable")
		return
	}
	// Both fields come from one config here so they can never be read from
	// different sources: what the persona claims, and what the binary that
	// config resolves to reports for itself. The advertised value is what the
	// persona actually presents — the probed version when browser.version is
	// unset — so page and status agree.
	status.AdvertisedBrowserVersion = stealth.ResolveBrowserVersion(h.Config)
	status.BrowserBinaryVersion = stealth.ProbedBinaryVersion(h.Config)
	if tabID := r.URL.Query().Get("tabId"); tabID != "" {
		if tracker, ok := h.Bridge.(interface{ FingerprintRotateActive(string) bool }); ok {
			status.TabOverrides["fingerprintRotateActive"] = tracker.FingerprintRotateActive(tabID)
		}
	}
	httpx.JSON(w, 200, status)
}
