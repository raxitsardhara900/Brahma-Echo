package bridge

import (
	"context"
	"strings"
	"testing"

	bridgecdpops "github.com/pinchtab/pinchtab/internal/bridge/cdpops"
)

// The validator is not gated on the action kind, so every kind that can carry a button is
// covered without a list anyone has to maintain — including one nobody thought of. These
// kinds are the ones the CLI and the pointer helpers actually pass a button for.
func TestValidateButtonActionRefusesAnUnknownNameForEveryKind(t *testing.T) {
	for _, kind := range []string{ActionMouseDown, ActionMouseUp, ActionDrag, ActionClick, ActionDoubleClick, "a-kind-nobody-listed"} {
		err := ValidateButtonAction(kind, ActionRequest{Kind: kind, Button: "rihgt"})
		if err == nil {
			t.Errorf("%s accepted a misspelled button, so it dispatches left and reports success", kind)
			continue
		}
		if !strings.Contains(err.Error(), "rihgt") {
			t.Errorf("%s refusal = %v, want it to name what the caller sent", kind, err)
		}

		for _, ok := range append(bridgecdpops.MouseButtons(), "", "RIGHT", " right ") {
			if err := ValidateButtonAction(kind, ActionRequest{Kind: kind, Button: ok}); err != nil {
				t.Errorf("%s refused %q: %v", kind, ok, err)
			}
		}
	}
}

// ExecuteAction is the second enforcement point: the handler refuses first with a 400, but a
// caller reaching the bridge directly must not get the silent left-click either.
func TestExecuteActionRefusesAnUnknownButtonBeforeDispatch(t *testing.T) {
	dispatched := false
	b := &Bridge{Actions: map[string]ActionFunc{
		ActionMouseDown: func(ctx context.Context, req ActionRequest) (map[string]any, error) {
			dispatched = true
			return map[string]any{"down": true}, nil
		},
	}}

	_, err := b.ExecuteAction(context.Background(), ActionMouseDown, ActionRequest{Button: "secondary"})

	if err == nil {
		t.Fatal("ExecuteAction accepted an unknown button")
	}
	if dispatched {
		t.Error("the action ran before the button was validated, so the refusal came too late to prevent the wrong click")
	}
	for _, name := range bridgecdpops.MouseButtons() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("refusal = %v, want it to name the valid button %q", err, name)
		}
	}
}

func TestExecuteActionStillDispatchesTheValidButtons(t *testing.T) {
	for _, button := range append(bridgecdpops.MouseButtons(), "", "RIGHT", " middle ") {
		dispatched := false
		b := &Bridge{Actions: map[string]ActionFunc{
			ActionMouseDown: func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				dispatched = true
				return map[string]any{"down": true}, nil
			},
		}}

		if _, err := b.ExecuteAction(context.Background(), ActionMouseDown, ActionRequest{Button: button}); err != nil {
			t.Errorf("button %q was refused: %v", button, err)
		}
		if !dispatched {
			t.Errorf("button %q did not reach the action", button)
		}
	}
}
