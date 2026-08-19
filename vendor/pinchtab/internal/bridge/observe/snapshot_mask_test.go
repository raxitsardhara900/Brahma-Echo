package observe

import (
	"encoding/json"
	"testing"
)

func passwordFieldNodes(axValue string) []RawAXNode {
	pass := RawAXNode{
		NodeID:           "pass",
		Role:             &RawAXValue{Value: json.RawMessage(`"textbox"`)},
		Name:             &RawAXValue{Value: json.RawMessage(`"Password"`)},
		BackendDOMNodeID: 11,
		Properties: []RawAXProp{
			{Name: "autocomplete", Value: &RawAXValue{Value: json.RawMessage(`"current-password"`)}},
		},
	}
	if axValue != "" {
		raw, _ := json.Marshal(axValue)
		pass.Value = &RawAXValue{Value: raw}
	}
	return []RawAXNode{
		{
			NodeID:   "root",
			Role:     &RawAXValue{Value: json.RawMessage(`"WebArea"`)},
			Name:     &RawAXValue{Value: json.RawMessage(`"Login"`)},
			ChildIDs: []string{"pass"},
		},
		pass,
	}
}

// This pass has no DOM access and runs before the best-effort enrichment that
// knows whether the field is empty, so it masks unconditionally: if enrichment
// never runs, this is the only thing keeping a password out of the snapshot.
func TestBuildSnapshotMasksSensitiveFieldsUnconditionally(t *testing.T) {
	tests := []struct {
		name    string
		axValue string
	}{
		{name: "filled", axValue: "supersecret123"},
		{name: "accessibility value suppressed", axValue: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, _ := BuildSnapshot(passwordFieldNodes(tt.axValue), "", -1)
			if len(flat) != 2 {
				t.Fatalf("expected 2 nodes, got %d", len(flat))
			}
			if flat[1].Value != MaskedValue {
				t.Fatalf("value = %q, want %q", flat[1].Value, MaskedValue)
			}
		})
	}
}

// A fixed-width mask cannot leak the secret's length; snapshots are persisted
// and rendered elsewhere, so length-proportional masking would widen what they
// expose about a secret.
func TestBuildSnapshotMaskWidthIsIndependentOfContent(t *testing.T) {
	short, _ := BuildSnapshot(passwordFieldNodes("ab"), "", -1)
	long, _ := BuildSnapshot(passwordFieldNodes("a-very-long-passphrase-indeed"), "", -1)

	if short[1].Value != long[1].Value {
		t.Fatalf("mask varies with content: %q vs %q", short[1].Value, long[1].Value)
	}
}

func TestIsSensitiveAutocomplete(t *testing.T) {
	sensitive := []string{"current-password", "new-password", "New-Password", "  current-password  "}
	for _, v := range sensitive {
		if !IsSensitiveAutocomplete(v) {
			t.Errorf("IsSensitiveAutocomplete(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "off", "username", "one-time-code"} {
		if IsSensitiveAutocomplete(v) {
			t.Errorf("IsSensitiveAutocomplete(%q) = true, want false", v)
		}
	}
}
