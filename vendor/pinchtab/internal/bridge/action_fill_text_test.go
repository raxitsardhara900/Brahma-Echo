package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fill reads Text while select reads Value, and no surface declared which — so a caller
// sending the other spelling wrote nothing and was told it had succeeded. FillText is the
// one owner of that resolution, and the second return is what separates "clear this field"
// from "the text never arrived", which used to be the same request.
func TestFillTextResolvesEitherSpellingAndReportsWhetherAnyWasSupplied(t *testing.T) {
	for _, tc := range []struct {
		name         string
		req          ActionRequest
		want         string
		wantSupplied bool
	}{
		{"text", ActionRequest{Text: "FILLED"}, "FILLED", true},
		{"value", ActionRequest{Value: "FILLED"}, "FILLED", true},
		{"both, text wins", ActionRequest{Text: "FROM_TEXT", Value: "FROM_VALUE"}, "FROM_TEXT", true},
		{"explicit empty text clears", ActionRequest{HasText: true}, "", true},
		{"nothing supplied", ActionRequest{}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, supplied := FillText(tc.req)
			if got != tc.want {
				t.Errorf("FillText() text = %q, want %q", got, tc.want)
			}
			if supplied != tc.wantSupplied {
				t.Errorf("FillText() supplied = %v, want %v", supplied, tc.wantSupplied)
			}
		})
	}
}

// The pair the whole card turns on: clearing stays legal, and a fill that carries no text
// under any spelling is refused instead of answering filled:true with len:0.
func TestValidateFillActionRefusesOnlyWhenNoTextWasSupplied(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    string
		req     ActionRequest
		wantErr bool
	}{
		{"nothing supplied", ActionFill, ActionRequest{Selector: "#q"}, true},
		{"explicit clear", ActionFill, ActionRequest{Selector: "#q", HasText: true}, false},
		{"text", ActionFill, ActionRequest{Selector: "#q", Text: "X"}, false},
		{"value only", ActionFill, ActionRequest{Selector: "#q", Value: "X"}, false},
		{"another kind is not fill's business", ActionType, ActionRequest{Selector: "#q"}, false},
		{"select keeps its own guard", ActionSelect, ActionRequest{Selector: "#q"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFillAction(tc.kind, tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want a refusal: this request writes nothing and used to report success")
				}
				for _, want := range []string{"text", "value", `"text": ""`} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal = %q, want it to mention %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateFillAction() error = %v, want none", err)
			}
		})
	}
}

// Presence has to survive the wire and the internal forwarding hop, which is why Text and
// Value are omitempty: re-marshaling used to re-introduce "text":"" and make every
// forwarded request look supplied, exactly as the X/Y comment describes for coordinates.
func TestTextPresenceSurvivesDecodeAndReMarshal(t *testing.T) {
	for _, tc := range []struct {
		name         string
		body         string
		wantSupplied bool
		wantText     string
	}{
		{"text supplied", `{"kind":"fill","text":"X"}`, true, "X"},
		{"empty text supplied", `{"kind":"fill","text":""}`, true, ""},
		{"empty value supplied", `{"kind":"fill","value":""}`, true, ""},
		{"nothing supplied", `{"kind":"fill","nodeId":7}`, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req ActionRequest
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			text, supplied := FillText(req)
			if supplied != tc.wantSupplied || text != tc.wantText {
				t.Fatalf("decoded FillText() = (%q, %v), want (%q, %v)", text, supplied, tc.wantText, tc.wantSupplied)
			}

			forwarded, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var again ActionRequest
			if err := json.Unmarshal(forwarded, &again); err != nil {
				t.Fatalf("re-Unmarshal() error = %v", err)
			}
			text, supplied = FillText(again)
			if supplied != tc.wantSupplied || text != tc.wantText {
				t.Fatalf("after re-marshal FillText() = (%q, %v), want (%q, %v); forwarded body was %s", text, supplied, tc.wantText, tc.wantSupplied, forwarded)
			}
		})
	}
}

// ExecuteAction is the in-process entry, so the refusal has to happen there too — and
// before dispatch, since the action itself cannot tell the two empty requests apart.
func TestExecuteActionRefusesAFillWithNoTextBeforeDispatch(t *testing.T) {
	b := &Bridge{}
	b.InitActionRegistry()

	_, err := b.ExecuteAction(context.Background(), ActionFill, ActionRequest{Selector: "#q"})
	if err == nil || !strings.Contains(err.Error(), "fill requires") {
		t.Fatalf("ExecuteAction() error = %v, want the missing-text refusal", err)
	}

	// The control: an explicit clear must get past validation and fail later, at the browser
	// it has no connection to — never with the refusal above.
	_, err = b.ExecuteAction(context.Background(), ActionFill, ActionRequest{Selector: "#q", HasText: true})
	if err != nil && strings.Contains(err.Error(), "fill requires") {
		t.Errorf("an explicit clear was refused as missing text: %v", err)
	}
}
