package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// upstreamRecorder records every request that reaches PinchTab, so a test can
// assert a rejected call never got there — an error result alone cannot tell a
// pre-dispatch rejection from a request that went out and came back failing.
func upstreamRecorder(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	paths := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*paths = append(*paths, r.Method+" "+r.URL.Path)
		resp := map[string]any{"path": r.URL.Path}
		if body, _ := io.ReadAll(r.Body); len(body) > 0 {
			var parsed map[string]any
			if json.Unmarshal(body, &parsed) == nil {
				resp["body"] = parsed
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, paths
}

// A malformed delta must be reported, not dropped. Dropping it degrades a wheel
// scroll into a bare scroll with no magnitude, because hasDeltaY gates the wheel
// branch.
func TestScrollRejectsAMalformedDeltaWithoutCallingUpstream(t *testing.T) {
	srv, paths := upstreamRecorder(t)

	for _, malformed := range []string{"-300px", "300 pixels", "three hundred"} {
		t.Run(malformed, func(t *testing.T) {
			*paths = nil
			result := callTool(t, "pinchtab_scroll", map[string]any{"deltaY": malformed}, srv)

			if !result.IsError {
				t.Fatalf("deltaY=%q was accepted: %s", malformed, resultText(t, result))
			}
			message := resultText(t, result)
			if !strings.Contains(message, "deltaY") {
				t.Errorf("error %q does not name the argument", message)
			}
			if !strings.Contains(message, malformed) {
				t.Errorf("error %q does not echo the received value", message)
			}
			if len(*paths) != 0 {
				t.Errorf("rejected call still reached upstream: %v", *paths)
			}
		})
	}
}

// The case a caller cannot detect: direction synthesises a magnitude precisely
// because the malformed deltaY was dropped, so the tool scrolls DOWN by 120 when
// asked to scroll UP by 300 — sign inverted, magnitude invented, no error.
func TestScrollRejectsAMalformedDeltaRatherThanLettingDirectionInventOne(t *testing.T) {
	srv, paths := upstreamRecorder(t)

	result := callTool(t, "pinchtab_scroll", map[string]any{
		"deltaY":    "-300px",
		"direction": "down",
	}, srv)

	if !result.IsError {
		body := resultText(t, result)
		if strings.Contains(body, "120") {
			t.Fatalf("direction invented a magnitude for a malformed deltaY: %s", body)
		}
		t.Fatalf("malformed deltaY with direction was accepted: %s", body)
	}
	if len(*paths) != 0 {
		t.Errorf("rejected call still reached upstream: %v", *paths)
	}
}

// withBounds is an opt-out, so a dropped "no" leaves bounds switched on and the
// response looks like the default rather than like the request.
func TestCaptureRejectsAMalformedBoolean(t *testing.T) {
	srv, paths := upstreamRecorder(t)

	result := callTool(t, "pinchtab_capture", map[string]any{"withBounds": "no"}, srv)

	if !result.IsError {
		t.Fatalf(`withBounds="no" was accepted: %s`, resultText(t, result))
	}
	message := resultText(t, result)
	if !strings.Contains(message, "withBounds") {
		t.Errorf("error %q does not name the argument", message)
	}
	if !strings.Contains(message, "no") {
		t.Errorf("error %q does not echo the received value", message)
	}
	if len(*paths) != 0 {
		t.Errorf("rejected call still reached upstream: %v", *paths)
	}
}

// Models emit "" for "not set"; turning that into a failure would be a
// regression, and so would rejecting an argument nobody passed.
func TestAbsentAndEmptyTypedArgumentsAreNotRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{name: "absent", args: map[string]any{}},
		{name: "empty string", args: map[string]any{"deltaY": "", "steps": "", "x": ""}},
		{name: "explicit null", args: map[string]any{"deltaY": nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateTypedArgs("pinchtab_scroll", tc.args); err != nil {
				t.Errorf("validateTypedArgs(%v) = %v, want nil — not set must not become a failure", tc.args, err)
			}
		})
	}
}

// Every shape the accessors read today must still pass validation, or the fix
// would break working callers rather than malformed ones.
func TestReadableTypedArgumentsPassValidation(t *testing.T) {
	for _, args := range []map[string]any{
		{"deltaY": float64(-300)},
		{"deltaY": "-300"},
		{"deltaY": " -300 "},
		{"deltaY": "-300.5"},
		{"steps": "2", "x": "10", "y": float64(20)},
	} {
		if err := validateTypedArgs("pinchtab_scroll", args); err != nil {
			t.Errorf("validateTypedArgs(%v) = %v, want nil", args, err)
		}
	}
	for _, args := range []map[string]any{
		{"withBounds": true},
		{"withBounds": "true"},
		{"withBounds": "false"},
		{"withBounds": "1"},
	} {
		if err := validateTypedArgs("pinchtab_capture", args); err != nil {
			t.Errorf("validateTypedArgs(%v) = %v, want nil", args, err)
		}
	}
}

// The argument list is derived from the schemas, so a tool gaining a WithNumber
// argument is validated on arrival. This asserts the derivation actually found
// the declared types rather than silently returning an empty map, which would
// make every test above pass vacuously.
func TestTypedArgsAreDerivedFromTheToolSchemas(t *testing.T) {
	types := schemaArgTypesOnce()
	if len(types) == 0 {
		t.Fatal("no tool schemas parsed — validation would be a no-op for every tool")
	}

	declared := 0
	for _, tool := range allTools() {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", tool.Name, err)
		}
		var schema struct {
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("unmarshal %s schema: %v", tool.Name, err)
		}
		for name, property := range schema.Properties {
			switch property.Type {
			case "number", "integer", "boolean":
				declared++
				if got := types[tool.Name][name]; got != property.Type {
					t.Errorf("%s.%s typed %q by the schema but %q by the validator", tool.Name, name, property.Type, got)
				}
			default:
				if got, ok := types[tool.Name][name]; ok {
					t.Errorf("%s.%s is %q in the schema but the validator typed it %q", tool.Name, name, property.Type, got)
				}
			}
		}
	}
	if declared == 0 {
		t.Fatal("no numeric or boolean argument found in any schema — this guard is checking nothing")
	}
	t.Logf("validating %d schema-declared numeric/boolean arguments", declared)
}

// A handler with no schema would silently skip validation, so a new tool cannot
// be added to one side only.
func TestEveryHandlerHasASchemaAndEverySchemaAHandler(t *testing.T) {
	handlers := rawHandlerMap(NewClient("http://example.invalid", ""))
	schemas := map[string]struct{}{}
	for _, tool := range allTools() {
		schemas[tool.Name] = struct{}{}
	}

	for name := range handlers {
		if _, ok := schemas[name]; !ok {
			t.Errorf("handler %q has no tool schema, so its arguments are never validated", name)
		}
	}
	for name := range schemas {
		if _, ok := handlers[name]; !ok {
			t.Errorf("tool %q has a schema but no handler", name)
		}
	}
}

// Every argument the package reads with a TYPED accessor must be declared as
// number/integer/boolean in some tool's schema. Two things follow from a missing
// declaration: a model cannot discover the argument, and validateTypedArgs cannot
// see it either — it derives from the schemas, so an undeclared argument keeps the
// silent-coercion-drop this package otherwise rejects.
//
// A UNION check, not a per-tool one: the declared set is built across ALL tools,
// so one declaration anywhere licenses the argument for every handler that reads
// it. Over a shared handler that is structurally blind to the case this package
// had twice — nodeId and x/y were each read for all nine action tools while only
// three declared them, and both passed here. TestNoActionToolIsSentATypedArgumentItDoesNotDeclare covers
// that per-tool half behaviourally, for the action family. The two are not one
// check: this one spans every handler in the package and catches a name declared
// nowhere at all; that one is narrower in scope and stricter within it.
//
// optString is deliberately NOT scanned. A string argument read without a
// declaration is undiscoverable but not a correctness hazard: there is no
// coercion, so nothing is silently dropped. Widening this census to optString
// would produce false positives on the many string aliases the handlers accept.
func TestEveryTypedAccessorArgumentIsDeclaredInASchema(t *testing.T) {
	declared := map[string]string{}
	for _, tool := range allTools() {
		for name, kind := range typedArgsOf(tool) {
			declared[name] = kind
		}
	}
	if len(declared) == 0 {
		t.Fatal("no typed arguments found in any schema — this census is checking nothing")
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	accessor := regexp.MustCompile(`opt(?:Int|Float|Bool)\(r, "([^"]+)"\)`)

	scanned, found := 0, 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, match := range accessor.FindAllStringSubmatch(string(raw), -1) {
			found++
			arg := match[1]
			if _, ok := declared[arg]; !ok {
				t.Errorf("%s reads %q with a typed accessor but no tool schema declares it as number/integer/boolean — "+
					"a model cannot discover it and validateTypedArgs cannot validate it, so a malformed value is silently dropped", name, arg)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no Go source scanned — this census is checking nothing")
	}
	if found == 0 {
		t.Fatal("no typed accessor call found — the pattern no longer matches how arguments are read")
	}
	t.Logf("checked %d typed-accessor call sites against %d declared typed arguments", found, len(declared))
}

// Declaring humanize is only half the fix: it was previously read solely to raise
// the mutual-exclusion error and never forwarded, so humanized input was
// unreachable from MCP even for a caller who guessed the name.
func TestPointerToolsForwardHumanizeToUpstream(t *testing.T) {
	for _, tool := range []string{"pinchtab_click", "pinchtab_hover"} {
		t.Run(tool, func(t *testing.T) {
			srv, _ := upstreamRecorder(t)

			result := callTool(t, tool, map[string]any{"selector": "#b", "humanize": true}, srv)
			if result.IsError {
				t.Fatalf("humanize=true was rejected: %s", resultText(t, result))
			}
			body, _ := resultJSON(t, result)["body"].(map[string]any)
			if got, ok := body["humanize"]; !ok || got != true {
				t.Errorf("outbound body humanize = %v (present: %v), want true — the argument must be usable, not just accepted", got, ok)
			}
		})
	}
}

// An explicit false is an opt-OUT and must travel, because the wire field is a
// per-request override of the instance default rather than a flag.
func TestPointerToolsForwardAnExplicitHumanizeFalse(t *testing.T) {
	srv, _ := upstreamRecorder(t)

	result := callTool(t, "pinchtab_click", map[string]any{"selector": "#b", "humanize": false}, srv)
	body, _ := resultJSON(t, result)["body"].(map[string]any)
	if got, ok := body["humanize"]; !ok || got != false {
		t.Errorf("outbound body humanize = %v (present: %v), want false to travel as an opt-out", got, ok)
	}
}

// Forwarding must not become an unconditional default: a call that omits humanize
// has to leave the instance config in charge.
func TestOmittedHumanizeIsNotForwarded(t *testing.T) {
	for _, tool := range []string{"pinchtab_click", "pinchtab_hover"} {
		t.Run(tool, func(t *testing.T) {
			srv, _ := upstreamRecorder(t)

			result := callTool(t, tool, map[string]any{"selector": "#b"}, srv)
			body, _ := resultJSON(t, result)["body"].(map[string]any)
			if got, ok := body["humanize"]; ok {
				t.Errorf("outbound body carries humanize = %v with none requested; the instance default must stay in charge", got)
			}
		})
	}
}

// The third defect: before humanize was declared, the schema-derived validator
// could not see it, so "yes" was silently dropped and the mutual-exclusion guard
// it feeds was bypassed — the guard fired for true and "true" but not "yes".
// argcheck.go is untouched; the declaration alone brings this under validation.
func TestMalformedHumanizeIsRejectedFromTheDeclarationAlone(t *testing.T) {
	srv, paths := upstreamRecorder(t)

	result := callTool(t, "pinchtab_click", map[string]any{"selector": "#b", "mode": "dom", "humanize": "yes"}, srv)
	if !result.IsError {
		t.Fatalf(`humanize="yes" was accepted: %s`, resultText(t, result))
	}
	message := resultText(t, result)
	if !strings.Contains(message, "humanize") {
		t.Errorf("error %q does not name the argument", message)
	}
	if len(*paths) != 0 {
		t.Errorf("rejected call still reached upstream: %v", *paths)
	}
}

// The mutual-exclusion rule the CLI enforces stays enforced, for the parsed string
// form too, and only on the tool that has both arguments.
func TestModeAndHumanizeRemainMutuallyExclusiveOnClick(t *testing.T) {
	for _, humanize := range []any{true, "true"} {
		srv, _ := upstreamRecorder(t)
		result := callTool(t, "pinchtab_click", map[string]any{"selector": "#b", "mode": "dom", "humanize": humanize}, srv)
		if !result.IsError {
			t.Errorf("mode+humanize=%v was accepted: %s", humanize, resultText(t, result))
		}
	}
}

// Only the pointer tools declare it. A tool without a pointer path must not, or
// the schema would advertise an argument its handler ignores.
func TestOnlyPointerToolsDeclareHumanize(t *testing.T) {
	declaring := map[string]bool{}
	for _, tool := range allTools() {
		if _, ok := typedArgsOf(tool)["humanize"]; ok {
			declaring[tool.Name] = true
		}
	}
	want := map[string]bool{"pinchtab_click": true, "pinchtab_hover": true}
	if !reflect.DeepEqual(declaring, want) {
		t.Errorf("tools declaring humanize = %v, want %v (the MCP members of the CLI addPointerActionFlags set)", declaring, want)
	}
	for kind := range humanizeAction {
		if !declaring["pinchtab_"+kind] {
			t.Errorf("handler forwards humanize for %q but pinchtab_%s does not declare it", kind, kind)
		}
	}
}

// argumentProbe is what this guard sends: the typed arguments to probe together,
// plus fixed-value companions that make them reachable at all.
//
// A probe is a SET rather than a name because some arguments cannot show an effect
// alone — resolveXY needs both x and y, and steps is folded in only under a
// direction, and only by multiplying into deltaY. Companions may be of any type:
// direction is a string, and this guard deliberately never probes strings (the
// sibling census records why — no coercion, so nothing is silently dropped), so a
// string can only ever be a companion. A combination-only argument sent as a
// singleton cannot produce an effect on any tool, which is a vacuous pass; the
// positive control below is what forces it to be declared here instead.
type argumentProbe struct {
	args       []string
	companions map[string]any
}

func (p argumentProbe) label() string { return strings.Join(p.args, "+") }

// combinationProbes are the probes whose arguments the handler only reads as a set.
// Everything else is derived as a singleton from the tools' own declarations.
//
// THE BOUNDARY, and it is deliberate. Each probe is sent alone or with the companions
// listed here, so the guard cannot see a leak that fires ONLY alongside an argument it
// does not pair with — pixels gated on direction reaches every tool while both halves
// of this test stay green, because the control only asks whether pixels can ever have
// an effect (it can, on scroll, alone) and the sweep never sends direction with it.
// The singleton form of the same leak reds immediately; the mutations recorded on the
// card bracket that difference on both sides.
//
// Enumerating the gap is combinatorial: the escape is per (probe, companion) PAIR, not
// per argument, so it grows with every typed argument added. The cheap alternative —
// one maximal probe per tool with every typed argument set at once — was considered and
// declined, because it does not catch this class either: the companion is frequently a
// STRING (direction here, mode elsewhere), and this guard never probes strings by
// design, since nothing is coerced and so nothing is silently dropped. Extending the
// maximal probe to strings needs a plausible value per string argument, which is a new
// hand-maintained map — the staleness this guard is built to avoid, in a different
// denomination.
//
// THE PRECONDITION is what makes that acceptable rather than ignored. A leak of this
// shape needs a typed argument read BEFORE the kind switch in handleAction that is
// neither declared by every action tool nor gated by a kind set. All three current
// pre-switch reads are covered by one of those, measured rather than assumed:
//
//	x, y    gated by xyAction, whose members are exactly the three action tools that
//	        declare x/y (click, hover, scroll); the other six never reach the read
//	nodeId  ungated, but declared by all nine action tools, so there is no tool it
//	        can leak to
//	tabId   read pre-switch and declared on every action tool, but as a string, which
//	        this guard's alphabet excludes for the reason above
//
// Every other typed argument is read inside its own case, where kind gating prevents a
// cross-tool leak by construction. So a reviewer's trigger is specific: a FOURTH
// pre-switch typed read, with neither mechanism, is what would make this gap reachable.
// Until then it is a hole in front of a shape the file does not contain, and this is
// where the chain of cards over this guard stops — not for lack of a next step, but
// because the next step costs more than the hazard.
var combinationProbes = []argumentProbe{
	{args: []string{"x", "y"}},
	{args: []string{"steps"}, companions: map[string]any{"direction": "down"}},
}

// callOutcome is everything this guard can observe about one tool call. The request
// list is recorded before the body is parsed: snap's effect is a second /snapshot
// GET, which makes the result text two concatenated JSON objects that resultJSON
// cannot parse — so a two-request call keeps its raw text and is compared on that.
type callOutcome struct {
	requests []string
	body     map[string]any
	text     string
}

func observeToolCall(t *testing.T, tool string, args map[string]any) callOutcome {
	t.Helper()
	srv, paths := upstreamRecorder(t)
	result := callTool(t, tool, args, srv)
	if result.IsError {
		t.Fatalf("%s rejected %v outright (%s); this guard reasons about arguments that are ignored, not rejected", tool, args, resultText(t, result))
	}
	outcome := callOutcome{requests: append([]string(nil), *paths...), text: resultText(t, result)}
	if len(outcome.requests) == 1 {
		outcome.body, _ = resultJSON(t, result)["body"].(map[string]any)
	}
	return outcome
}

// describeDifference reports how two outcomes differ, or "" when they do not. This
// is the guard's whole oracle: an argument had an observable effect if ANYTHING the
// caller can see changed. Matching the probe's own value instead — which is what
// this replaced — was blind to a derived effect, and needed hasXY hand-added to the
// condition as the tell.
func describeDifference(baseline, probed callOutcome) string {
	if !reflect.DeepEqual(baseline.requests, probed.requests) {
		return fmt.Sprintf("upstream requests %v -> %v", baseline.requests, probed.requests)
	}
	if baseline.body == nil || probed.body == nil {
		if baseline.text != probed.text {
			return "the response text changed"
		}
		return ""
	}
	var changes []string
	for field, want := range baseline.body {
		got, present := probed.body[field]
		switch {
		case !present:
			changes = append(changes, fmt.Sprintf("%s=%v dropped", field, want))
		case !reflect.DeepEqual(want, got):
			changes = append(changes, fmt.Sprintf("%s %v -> %v", field, want, got))
		}
	}
	for field, got := range probed.body {
		if _, present := baseline.body[field]; !present {
			changes = append(changes, fmt.Sprintf("%s=%v added", field, got))
		}
	}
	sort.Strings(changes)
	return strings.Join(changes, ", ")
}

// The per-tool half the name-level census above cannot see. A typed argument read
// for a kind whose tool does not declare it is undiscoverable AND unvalidated,
// because validateTypedArgs keys its type map per tool — the name being declared
// somewhere else does not help the tool being called. handleAction reads its
// arguments before switching on kind, so this is where that goes wrong.
//
// Behavioural rather than structural: reachability is observed as a BASELINE DIFF —
// the same call with and without the probe — so the sentinel, a value derived from
// it, a dropped field and an extra upstream request all count uniformly, and no
// derivative has to be listed by hand. That needs no tool-to-handler-source mapping
// and cannot go stale as the handler moves code around.
//
// Its blind spot is an undeclared read with no observable effect at all, which by
// construction changes nothing a caller can see.
func TestNoActionToolIsSentATypedArgumentItDoesNotDeclare(t *testing.T) {
	probes := append([]argumentProbe(nil), combinationProbes...)
	grouped := map[string]bool{}
	for _, probe := range probes {
		for _, name := range probe.args {
			grouped[name] = true
		}
	}
	for _, tc := range actionToolTargets {
		for name := range schemaArgTypesOnce()[tc.tool] {
			if !grouped[name] {
				probes = append(probes, argumentProbe{args: []string{name}})
				grouped[name] = true
			}
		}
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].label() < probes[j].label() })

	// The declared type of each name, taken from whichever action tool declares it,
	// so the probe value is one the accessor would actually read.
	typeOf := map[string]string{}
	for _, tc := range actionToolTargets {
		for name, kind := range schemaArgTypesOnce()[tc.tool] {
			typeOf[name] = kind
		}
	}

	const sentinel = 424242.0
	probeArgs := func(tool string, probe argumentProbe, withProbe bool) map[string]any {
		extra := map[string]any{"selector": "#a"}
		for name, value := range probe.companions {
			extra[name] = value
		}
		if withProbe {
			for _, name := range probe.args {
				if typeOf[name] == "boolean" {
					extra[name] = true
					continue
				}
				extra[name] = sentinel
			}
		}
		return actionArgs(tool, extra)
	}

	// A diff oracle is only as trustworthy as the calls it compares. If anything in
	// an outcome varied between two identical calls, every future card would inherit
	// a red here — so this fails loudly and names the fields rather than normalising
	// them away, and an exclusion has to be added deliberately.
	for _, tc := range actionToolTargets {
		for _, probe := range probes {
			first := observeToolCall(t, tc.tool, probeArgs(tc.tool, probe, false))
			second := observeToolCall(t, tc.tool, probeArgs(tc.tool, probe, false))
			if diff := describeDifference(first, second); diff != "" {
				t.Fatalf("%s is not stable across two identical calls (%s baseline): %s — the diff oracle below would read this as an effect, so exclude the varying field explicitly before trusting it",
					tc.tool, probe.label(), diff)
			}
		}
	}

	// The positive control, and the half that keeps the probe list self-maintaining:
	// every probe must be demonstrably capable of an effect on a tool that DOES
	// declare it. A combination-only argument fails this as a singleton, which is
	// what forces it into combinationProbes with the companions it needs instead of
	// sitting in the undeclared sweep proving nothing.
	for _, probe := range probes {
		declaring := ""
		for _, tc := range actionToolTargets {
			declared := schemaArgTypesOnce()[tc.tool]
			all := true
			for _, name := range probe.args {
				if _, ok := declared[name]; !ok {
					all = false
				}
			}
			if all {
				declaring = tc.tool
				break
			}
		}
		if declaring == "" {
			t.Errorf("no action tool declares %s, so the sweep below can never distinguish reachable from ignored for it", probe.label())
			continue
		}
		baseline := observeToolCall(t, declaring, probeArgs(declaring, probe, false))
		probed := observeToolCall(t, declaring, probeArgs(declaring, probe, true))
		if diff := describeDifference(baseline, probed); diff == "" {
			t.Errorf("%s has no observable effect on %s, which DECLARES it — so its rows in the sweep below prove nothing. Give it the companions that make it reachable (see combinationProbes) rather than leaving it a singleton.",
				probe.label(), declaring)
		}
	}

	checked := 0
	for _, tc := range actionToolTargets {
		declared := schemaArgTypesOnce()[tc.tool]
		for _, probe := range probes {
			kinds := map[string]int{}
			for _, name := range probe.args {
				kinds[declared[name]]++
			}
			if len(kinds) != 1 {
				t.Errorf("%s declares %s inconsistently (%v); the group must be all-or-nothing", tc.tool, probe.label(), kinds)
				continue
			}
			if _, isDeclared := declared[probe.args[0]]; isDeclared {
				continue
			}
			checked++

			baseline := observeToolCall(t, tc.tool, probeArgs(tc.tool, probe, false))
			probed := observeToolCall(t, tc.tool, probeArgs(tc.tool, probe, true))
			if diff := describeDifference(baseline, probed); diff != "" {
				t.Errorf("%s: %s is reachable but the tool does not declare it — %s, so it is undiscoverable in tools/list and validateTypedArgs cannot type-check it",
					tc.tool, probe.label(), diff)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no undeclared (tool, argument) pair exercised — this guard is checking nothing")
	}
	t.Logf("checked %d undeclared (tool, probe) pairs across %d action tools and %d probes", checked, len(actionToolTargets), len(probes))
}

// A wrapper index used to mean document order for css:/xpath: and semantic rank
// for text:, so nth:1 could resolve earlier in the page than nth:0. The rule is
// one rule now, and a schema that offers first/last/nth without stating it leaves
// an agent to infer the grammar from trial and error. Derived over the schemas:
// any tool that gains a wrapper-accepting selector inherits the requirement.
func TestSelectorSchemasThatOfferWrappersStateWhatAnIndexMeans(t *testing.T) {
	checked := 0
	for _, tool := range allTools() {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", tool.Name, err)
		}
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("unmarshal %s schema: %v", tool.Name, err)
		}
		for name, property := range schema.Properties {
			if !strings.Contains(property.Description, "first/last/nth") {
				continue
			}
			checked++
			if !strings.Contains(property.Description, "document order") {
				t.Errorf("%s.%s offers first/last/nth without saying an index follows document order: %q", tool.Name, name, property.Description)
			}
			if !strings.Contains(property.Description, "text:X and first:text:X can differ") {
				t.Errorf("%s.%s does not warn that a bare text: selector ranks rather than indexes: %q", tool.Name, name, property.Description)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no schema offers first/last/nth, so this guard checked nothing")
	}
}
