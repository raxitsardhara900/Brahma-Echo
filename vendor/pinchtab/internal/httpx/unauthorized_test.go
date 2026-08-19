package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnauthorizedDistinguishesItsThreeCases(t *testing.T) {
	hints := map[string]string{}
	for _, tc := range []struct {
		name      string
		code      string
		presented string
		wantSays  string
	}{
		{name: "missing token", code: CodeMissingToken, presented: "", wantSays: "PINCHTAB_TOKEN"},
		{name: "bad token", code: CodeBadToken, presented: "deadbeef", wantSays: "--server"},
		{name: "session token under the bearer scheme", code: CodeBadToken, presented: "ses_0123456789abcdef", wantSays: "Authorization: Session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			Unauthorized(w, tc.code, tc.presented)

			if w.Code != 401 {
				t.Fatalf("status = %d, want 401", w.Code)
			}
			wantHeader := `Bearer realm="pinchtab", error="` + tc.code + `"`
			if got := w.Header().Get("WWW-Authenticate"); got != wantHeader {
				t.Errorf("WWW-Authenticate = %q, want %q", got, wantHeader)
			}

			var body struct {
				Error   string `json:"error"`
				Code    string `json:"code"`
				Details struct {
					Hint string `json:"hint"`
				} `json:"details"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != tc.code || body.Error != "unauthorized" {
				t.Errorf("body code=%q error=%q, want %q/unauthorized", body.Code, body.Error, tc.code)
			}
			if !strings.Contains(body.Details.Hint, tc.wantSays) {
				t.Errorf("hint = %q, want it to carry %q", body.Details.Hint, tc.wantSays)
			}
			if strings.Contains(body.Details.Hint, "deadbeef") || strings.Contains(body.Details.Hint, "ses_0123456789abcdef") {
				t.Errorf("hint = %q echoes the presented credential", body.Details.Hint)
			}
			hints[tc.name] = body.Details.Hint

			var raw map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			if details, ok := raw["details"].(map[string]any); ok {
				if _, hasRemedy := details["remedy"]; hasRemedy {
					t.Error("a 401 carries a remedy; no credential guidance can be a runnable line, the value is a secret only the operator has")
				}
			}
		})
	}

	seen := map[string]string{}
	for name, hint := range hints {
		if other, dup := seen[hint]; dup {
			t.Errorf("%q and %q share one hint %q; the whole point is that the cases say different things", name, other, hint)
		}
		seen[hint] = name
	}
}
