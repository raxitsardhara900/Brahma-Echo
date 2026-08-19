package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisabledEndpointHandlerIncludesHintAndRemedy(t *testing.T) {
	handler := DisabledEndpointHandler("recording", "security.allowScreencast", "recording_disabled")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/record/start", nil)
	handler(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var resp struct {
		Error   string         `json:"error"`
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if resp.Code != "recording_disabled" {
		t.Fatalf("code = %q, want recording_disabled", resp.Code)
	}

	hint, _ := resp.Details["hint"].(string)
	remedy, _ := resp.Details["remedy"].(string)

	if hint == "" {
		t.Fatal("expected non-empty hint in details")
	}
	if remedy == "" {
		t.Fatal("expected non-empty remedy in details")
	}
	if remedy != "pinchtab config set security.allowScreencast true && pinchtab server restart" {
		t.Fatalf("remedy = %q, want the config set AND the restart that applies it", remedy)
	}
}

// The label is the capability, not the endpoint: /storage is gated by
// stateExport, and /record/* by the screencast setting. Calling either label an
// "endpoint" sends the reader looking for a route that does not exist.
func TestDisabledEndpointMessageNamesTheCapabilityNotAnEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		setting    string
	}{
		{"capability named after another endpoint", "stateExport", "security.allowStateExport"},
		{"capability matching its own endpoint", "cookies", "security.allowCookies"},
		{"feature gated by a differently named setting", "recording", "security.allowScreencast"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := DisabledEndpointMessage(tt.capability, tt.setting)

			if strings.Contains(msg, tt.capability+" endpoint") {
				t.Fatalf("message calls the capability an endpoint: %q", msg)
			}
			if !strings.Contains(msg, tt.capability+" capability") {
				t.Fatalf("message does not name the required capability: %q", msg)
			}
			if !strings.Contains(msg, tt.setting) {
				t.Fatalf("message does not name the setting to change: %q", msg)
			}
		})
	}
}

func TestDisabledEndpointHandlerKeepsSettingHintAndRemedy(t *testing.T) {
	handler := DisabledEndpointHandler("stateExport", "security.allowStateExport", "state_export_disabled")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/storage", nil)
	handler(w, r)

	var resp struct {
		Error   string         `json:"error"`
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != "state_export_disabled" {
		t.Fatalf("code = %q, want state_export_disabled", resp.Code)
	}
	if strings.Contains(resp.Error, "stateExport endpoint") {
		t.Fatalf("error still describes a stateExport endpoint: %q", resp.Error)
	}
	for key, want := range map[string]string{
		"setting": "security.allowStateExport",
		"hint":    "Enable security.allowStateExport to use this feature.",
		"remedy":  "pinchtab config set security.allowStateExport true && pinchtab server restart",
	} {
		if got, _ := resp.Details[key].(string); got != want {
			t.Fatalf("details[%q] = %q, want %q", key, got, want)
		}
	}
}

// The defect this pins: the remedy used to stop at the config write. Writing the
// setting is a successful no-op for the caller — the security block is read at
// boot, so the very same 403 comes back — and the caller reading this refusal is
// an agent that has no other instruction to try. The properties, not the
// sentence: both commands present, the config write FIRST, joined so a shell can
// run the string verbatim, and no prose asking the reader to interpret anything.
func TestDisabledEndpointRemedyIsOneRunnableLineEndingInTheRestart(t *testing.T) {
	for _, setting := range []string{"security.allowCookies", "security.allowStateExport", "security.allowClipboard"} {
		remedy, _ := DisabledEndpointDetails(setting)["remedy"].(string)

		configCmd := "pinchtab config set " + setting + " true"
		restartCmd := "pinchtab server restart"

		if strings.Contains(remedy, "\n") {
			t.Errorf("remedy for %s spans lines, so it cannot be run verbatim: %q", setting, remedy)
		}
		if !strings.Contains(remedy, configCmd) {
			t.Errorf("remedy for %s does not enable the setting: %q", setting, remedy)
		}
		if !strings.Contains(remedy, restartCmd) {
			t.Errorf("remedy for %s omits the restart, so following it verbatim returns the identical 403: %q", setting, remedy)
		}
		if strings.Index(remedy, configCmd) > strings.Index(remedy, restartCmd) {
			t.Errorf("remedy for %s restarts before writing the setting, so the restart applies nothing: %q", setting, remedy)
		}
		if !strings.Contains(remedy, "&&") {
			t.Errorf("remedy for %s joins its two commands with prose rather than a shell operator: %q", setting, remedy)
		}
		for _, prose := range []string{" then", "then:", "after that", "first ", "next "} {
			if strings.Contains(remedy, prose) {
				t.Errorf("remedy for %s contains prose %q an agent would have to interpret: %q", setting, prose, remedy)
			}
		}
	}
}
