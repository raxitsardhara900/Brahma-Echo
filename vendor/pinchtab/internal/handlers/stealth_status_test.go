package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

// fakeBrowserBinary writes an executable that answers --version the way a real
// browser does, so the probe has something true to read without a browser.
func fakeBrowserBinary(t *testing.T, version string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary uses a shell script")
	}
	path := filepath.Join(t.TempDir(), "fake-chrome")
	script := "#!/bin/sh\necho 'Google Chrome " + version + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func stealthStatusBody(t *testing.T, cfg *config.RuntimeConfig) map[string]any {
	t.Helper()
	h := New(&mockBridge{}, cfg, nil, nil, nil)
	req := httptest.NewRequest("GET", "/stealth/status", nil)
	w := httptest.NewRecorder()

	h.HandleStealthStatus(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return resp
}

// The whole point of the two fields: a caller can see the persona and the
// binary disagree instead of having to infer it from a page read.
func TestStealthStatusReportsAdvertisedAndRealBrowserVersions(t *testing.T) {
	resp := stealthStatusBody(t, &config.RuntimeConfig{
		BrowserVersion: "144.0.7559.133",
		BrowserBinary:  fakeBrowserBinary(t, "199.0.1234.5"),
	})

	if got := resp["advertisedBrowserVersion"]; got != "144.0.7559.133" {
		t.Errorf("advertisedBrowserVersion = %v, want the version the persona presents", got)
	}
	if got := resp["browserBinaryVersion"]; got != "199.0.1234.5" {
		t.Errorf("browserBinaryVersion = %v, want the version the launched binary reports", got)
	}
}

// Absence control: the mismatch above must come from the two values differing,
// not from the endpoint always reporting them as different.
func TestStealthStatusReportsAgreementWhenTheVersionsMatch(t *testing.T) {
	resp := stealthStatusBody(t, &config.RuntimeConfig{
		BrowserVersion: "199.0.1234.5",
		BrowserBinary:  fakeBrowserBinary(t, "199.0.1234.5"),
	})

	if resp["advertisedBrowserVersion"] != resp["browserBinaryVersion"] {
		t.Errorf("advertised %v vs binary %v; equal inputs must report equal",
			resp["advertisedBrowserVersion"], resp["browserBinaryVersion"])
	}
}

// With browser.version unset, the advertised version is the one the persona now
// actually presents — the probed binary version — so page and status agree. This
// is the wiring the fix adds: before it, advertised was the frozen fallback while
// the binary reported something newer.
func TestStealthStatusAdvertisesTheProbedVersionWhenConfigIsUnset(t *testing.T) {
	resp := stealthStatusBody(t, &config.RuntimeConfig{
		BrowserBinary: fakeBrowserBinary(t, "150.0.7871.187"),
	})

	if got := resp["advertisedBrowserVersion"]; got != "150.0.7871.187" {
		t.Errorf("advertisedBrowserVersion = %v, want the probed binary version when browser.version is unset", got)
	}
	if resp["advertisedBrowserVersion"] != resp["browserBinaryVersion"] {
		t.Errorf("advertised %v vs binary %v; with no explicit version the persona must advertise the probed one",
			resp["advertisedBrowserVersion"], resp["browserBinaryVersion"])
	}
}

// An unprobeable binary must read as unknown rather than as a disagreement,
// or every sandboxed deployment looks like the defect.
func TestStealthStatusOmitsTheBinaryVersionWhenTheProbeFails(t *testing.T) {
	resp := stealthStatusBody(t, &config.RuntimeConfig{
		BrowserVersion: "144.0.7559.133",
		BrowserBinary:  filepath.Join(t.TempDir(), "does-not-exist"),
	})

	if got, ok := resp["browserBinaryVersion"]; ok {
		t.Errorf("browserBinaryVersion = %v for an unrunnable binary, want the field omitted", got)
	}
	if got := resp["advertisedBrowserVersion"]; got != "144.0.7559.133" {
		t.Errorf("advertisedBrowserVersion = %v, want it reported regardless of the probe", got)
	}
}
