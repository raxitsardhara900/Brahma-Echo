package remedy

import (
	"strings"
	"testing"
)

// THE CENSUS THIS CARD WAS FILED ON, pinned as a fixture. Each of these shipped as a
// details.remedy while the CLI printed them all into one "Remedy:" slot, so a caller that
// learned the field was executable from one of them mis-executed the rest. They are here as
// the guard's red cases: if the validator ever accepts one again, the field has gone back to
// meaning four things.
func TestTheHistoricalRemediesAreRejected(t *testing.T) {
	for _, tc := range []struct {
		was  string
		line string
		why  string
	}{
		{
			was:  "two commands joined by the prose word then:",
			line: `pinchtab config set security.allowedDomains "$(pinchtab config get security.allowedDomains),example.com" then: pinchtab server restart`,
			why:  "a shell runs the first command with then: and pinchtab as arguments",
		},
		{
			was:  "a command with a prose tail carrying two alternatives",
			line: "pinchtab back (returns the tab to the previous allowed page; use pinchtab nav <allowed-url> when there is no history entry, or close the tab)",
			why:  "the parenthesis is a subshell, not an explanation",
		},
		{
			was:  "a command with a prose alternative",
			line: "pinchtab click <ref> --wait-nav (use --submit instead when the click submits a form)",
			why:  "same parenthesis, and the alternative is a second command",
		},
		{
			was:  "pure prose",
			line: "If TLS terminates in front of PinchTab, also enable server.trustProxyHeaders so forwarded https requests are recognized.",
			why:  "there is no command at all, so the field must be absent",
		},
		{
			was:  "a bare verb, with the command in the hint beside it",
			line: "download",
			why:  "a verb is not an invocation",
		},
		{
			was:  "two commands joined by prose, from the branch that had an executable sibling",
			line: "pinchtab session info (with PINCHTAB_SESSION set) prints the id, or pinchtab session list",
			why:  "one producer emitted this and an executable line depending on a branch",
		},
		{
			was:  "a verb leading the command",
			line: "run the full server instead: pinchtab server",
			why:  "the command is behind prose, so the line does not start one",
		},
		{
			was:  "prose describing a file edit",
			line: "set sessions.agent.enabled = true in config.json, then restart the server",
			why:  "no command can do this, so the field must be absent",
		},
		{
			was:  "alternatives offered with a pipe",
			line: "pinchtab dialog accept|dismiss (or pass --dialog-action accept|dismiss on the action that opens the dialog)",
			why:  "the worst of them: a shell reads this as a pipeline into a command named dismiss, so it is valid and wrong rather than merely broken",
		},
	} {
		t.Run(tc.was, func(t *testing.T) {
			if err := Validate(tc.line); err == nil {
				t.Fatalf("Validate accepted %q; %s", tc.line, tc.why)
			}
		})
	}
}

func TestTheShapesARemedyMayTake(t *testing.T) {
	for _, line := range []string{
		"pinchtab back",
		"pinchtab dialog accept",
		"pinchtab session revoke <session-id>",
		`pinchtab download "<url>"`,
		"pinchtab config set security.allowClipboard true && pinchtab server restart",
		`pinchtab config set security.allowedDomains "$(pinchtab config get security.allowedDomains),example.com" && pinchtab server restart`,
	} {
		if err := Validate(line); err != nil {
			t.Errorf("Validate rejected a remedy that a shell runs: %q: %v", line, err)
		}
	}
}

// A slot must be a <name>, because that is the convention the CLI's own help uses and the
// only one an agent can be expected to substitute into.
func TestASlotMustBeANamedPlaceholder(t *testing.T) {
	// A bare uppercase word (pinchtab nav URL) is deliberately NOT rejected here: no local
	// check can tell it from a literal argument value, and the command-tree guard is the
	// layer that answers whether the command accepts it.
	for _, line := range []string{
		"pinchtab nav {url}",
		"pinchtab nav <URL>",
		"pinchtab nav <>",
		"pinchtab nav 2>/dev/null",
	} {
		if Validate(line) == nil {
			t.Errorf("Validate accepted %q, whose free slot no caller can fill", line)
		}
	}
	if err := Validate("pinchtab nav <url>"); err != nil {
		t.Errorf("Validate rejected the conventional placeholder: %v", err)
	}
}

func TestFillInterpolatesAKnownValue(t *testing.T) {
	template := Declare(`pinchtab download "<url>"`)

	if got := template.Fill("https://example.com/a.zip"); got != `pinchtab download "https://example.com/a.zip"` {
		t.Errorf("Fill = %q, want the value interpolated", got)
	}
	if got := template.Remedy(); got != `pinchtab download "<url>"` {
		t.Errorf("Remedy = %q, want the slot intact", got)
	}
	// A value that would break the property yields no remedy rather than a line the caller
	// cannot run: absence is the honest answer, and the caller still has the hint.
	if got := template.Fill(`a" && rm -rf /`); !got.Empty() {
		t.Errorf("Fill = %q, want none for a value that changes what the line does", got)
	}
}

func TestFillPanicsOnMoreValuesThanSlots(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Fill accepted more values than the template has slots, so a value was silently dropped")
		}
	}()
	Declare("pinchtab session revoke <session-id>").Fill("a", "b")
}

func TestDeclarePanicsOnProse(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Declare accepted prose, so a producer can publish guidance no shell runs")
		}
	}()
	Declare("just restart the server")
}

// Absence is a statement, so Details omits rather than emitting an empty field: a client
// reading remedy learns there is no command, not that there is an empty one.
func TestDetailsOmitsWhatIsAbsent(t *testing.T) {
	both := Details("prose", Declare("pinchtab back").Remedy())
	if both["hint"] != "prose" || both["remedy"] != "pinchtab back" {
		t.Errorf("Details = %#v, want both fields", both)
	}

	hintOnly := Details("prose", None)
	if _, ok := hintOnly["remedy"]; ok {
		t.Errorf("Details = %#v, want no remedy key when there is no command", hintOnly)
	}
	if _, ok := Details("", None)["hint"]; ok {
		t.Error("Details emitted an empty hint")
	}
}

// Segments is what the command-tree guard resolves, so a quoted value with a nested
// substitution must arrive as ONE word: split naively, `config set` looks like it was handed
// six arguments and the guard reports a defect that is not there.
func TestSegmentsSplitTheWayAShellWould(t *testing.T) {
	line := `pinchtab config set security.allowedDomains "$(pinchtab config get security.allowedDomains),example.com" && pinchtab server restart`

	segments := Segments(line)
	if len(segments) != 2 {
		t.Fatalf("Segments = %#v, want the two &&-joined commands", segments)
	}
	if len(segments[0]) != 5 {
		t.Errorf("first command = %#v, want pinchtab config set <path> <value> as five words", segments[0])
	}
	if got := []string{"pinchtab", "server", "restart"}; strings.Join(segments[1], " ") != strings.Join(got, " ") {
		t.Errorf("second command = %#v, want %#v", segments[1], got)
	}
	if Segments("restart the server") != nil {
		t.Error("Segments returned commands for prose")
	}
}
