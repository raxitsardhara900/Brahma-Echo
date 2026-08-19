package scroll

import "testing"

// The vocabulary and the magnitude are asserted here, at the owner, because both
// surfaces now derive their behaviour from these values: a change made here moves the
// CLI and MCP together, which is the whole point of the package existing.
func TestDirectionsCoverBothAxesInBothSignsAtOneStep(t *testing.T) {
	want := map[string]Direction{
		"down":  {Axis: "scrollY", Delta: StepPixels},
		"up":    {Axis: "scrollY", Delta: -StepPixels},
		"right": {Axis: "scrollX", Delta: StepPixels},
		"left":  {Axis: "scrollX", Delta: -StepPixels},
	}

	for keyword, expected := range want {
		got, ok := DirectionFor(keyword)
		if !ok {
			t.Errorf("%q is not an accepted direction", keyword)
			continue
		}
		if got != expected {
			t.Errorf("DirectionFor(%q) = %+v, want %+v", keyword, got, expected)
		}
	}
	if len(DirectionKeywords()) != len(want) {
		t.Errorf("keywords = %v, want exactly the four in this table", DirectionKeywords())
	}
	if _, ok := DirectionFor("top"); ok {
		t.Error("'top' resolved; a scroll-to-extreme is an absolute position, not a keyword step")
	}
	if _, ok := DirectionFor("DOWN"); ok {
		t.Error("'DOWN' resolved here; case folding is the caller's job, so a caller that forgets it must fail rather than half-work")
	}
}

// A keyword step is a viewport-scale move, not a device notch. The browser's default
// wheel notch (internal/bridge defaultScrollNotch) answers a different question and
// stays 120; this asserts the two have not been collapsed into one value, which is what
// made "scroll down" move a sixth of a viewport through MCP.
func TestAKeywordStepIsNotAWheelNotch(t *testing.T) {
	const wheelNotch = 120
	if StepPixels == wheelNotch {
		t.Errorf("StepPixels = %d, the same as a wheel notch; a keyword is an intent and a notch is a device unit, so collapsing them reintroduces the six-round-trip scroll", StepPixels)
	}
	if StepPixels < wheelNotch {
		t.Errorf("StepPixels = %d, smaller than one wheel notch", StepPixels)
	}
}
