package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/config"
	"github.com/pinchtab/pinchtab/internal/routes"
)

// recordingMux captures registered patterns so a test can assert the live route
// set matches the shared catalog without standing up a real server.
type recordingMux struct{ patterns []string }

func (r *recordingMux) HandleFunc(pattern string, _ func(http.ResponseWriter, *http.Request)) {
	r.patterns = append(r.patterns, pattern)
}

// expectedRoutes derives the route set the bridge must register from the catalog
// plus the documented special cases: this is the single source of truth the
// registration is verified against.
func expectedRoutes() map[string]bool {
	want := map[string]bool{}
	for _, ep := range routes.Core() {
		if !tabOnlyRoutes[ep.Route()] {
			want[ep.Route()] = true
		}
		if ep.TabScoped {
			want[ep.TabRoute()] = true
		}
	}
	for _, p := range specialCaseRoutes {
		want[p] = true
	}
	return want
}

func TestRegisteredRoutesMatchCatalog(t *testing.T) {
	h := &Handlers{}
	rec := &recordingMux{}
	h.registerBridgeRoutes(rec)
	h.registerSpecialRoutes(rec, func() {})

	got := map[string]bool{}
	for _, p := range rec.patterns {
		if got[p] {
			t.Errorf("route %q registered more than once", p)
		}
		got[p] = true
	}

	want := expectedRoutes()

	for p := range got {
		if !want[p] {
			t.Errorf("registered route %q is not in the catalog or special-case list", p)
		}
	}
	for p := range want {
		if !got[p] {
			t.Errorf("expected route %q was not registered", p)
		}
	}
}

// TestTabOnlyRoutesHaveNoRootForm guards the handoff/resume behavior: they must
// be registered only in their /tabs/{id}/... form, never at the root.
func TestTabOnlyRoutesHaveNoRootForm(t *testing.T) {
	h := &Handlers{}
	rec := &recordingMux{}
	h.registerBridgeRoutes(rec)

	registered := map[string]bool{}
	for _, p := range rec.patterns {
		registered[p] = true
	}
	for route := range tabOnlyRoutes {
		if registered[route] {
			t.Errorf("tab-only route %q must not be registered at the root", route)
		}
	}
}

// TestShutdownRouteIsConditional confirms POST /shutdown registers only when a
// shutdown function is supplied.
func TestShutdownRouteIsConditional(t *testing.T) {
	h := &Handlers{}

	without := &recordingMux{}
	h.registerSpecialRoutes(without, nil)
	for _, p := range without.patterns {
		if p == "POST /shutdown" {
			t.Fatal("POST /shutdown registered with nil shutdown func")
		}
	}

	with := &recordingMux{}
	h.registerSpecialRoutes(with, func() {})
	found := false
	for _, p := range with.patterns {
		if p == "POST /shutdown" {
			found = true
		}
	}
	if !found {
		t.Fatal("POST /shutdown not registered when shutdown func supplied")
	}
}

func TestRegisteredRouteCount(t *testing.T) {
	h := &Handlers{}
	rec := &recordingMux{}
	h.registerBridgeRoutes(rec)
	h.registerSpecialRoutes(rec, func() {})

	if len(rec.patterns) != len(expectedRoutes()) {
		sort.Strings(rec.patterns)
		t.Fatalf("registered %d routes, expected %d", len(rec.patterns), len(expectedRoutes()))
	}
}

// gatedCatalogRoutes is the population this guard walks: every catalogue entry declaring a
// capability. It is enumerated from routes.Core(), never restated here, so a gated route
// added tomorrow is covered the day it lands rather than the day someone remembers.
func gatedCatalogRoutes(t *testing.T) []routes.Endpoint {
	t.Helper()

	var gated []routes.Endpoint
	seen := map[routes.Capability]bool{}
	for _, ep := range routes.Core() {
		if ep.Capability == routes.CapNone {
			continue
		}
		// A gated route registered only in its /tabs/{id}/ form has no root form to drive,
		// and there are none today. Failing here rather than skipping is deliberate: a
		// silently excluded route is exactly the gap this guard exists to close, so the
		// first one to appear forces the decision instead of vanishing from the population.
		if tabOnlyRoutes[ep.Route()] {
			t.Fatalf("%s is capability-gated and tab-only, so this guard cannot drive it; give it a root form or drive the tab form here — do not let it fall out of the population", ep.Route())
		}
		gated = append(gated, ep)
		seen[ep.Capability] = true
	}

	// THREE FLOORS, and each catches a different thing — stated exactly, because a guard whose
	// comment overstates its own coverage is what stops the next reader strengthening it.
	//
	//  1. The count floor is a HAND-PICKED number. It catches an enumeration that stopped
	//     matching wholesale. It cannot catch a population that shrinks by a few routes, and
	//     it is not evidence of anything above that number.
	//  2. CapabilityEndpoints() is a different FUNCTION but not a different SOURCE: it walks
	//     the same coreEndpoints slice through the same Capability != CapNone predicate. So it
	//     only bites when a capability loses ALL of its routes — de-declare one route of three
	//     and both this and the count floor stay green.
	//  3. endpointSecurityStates() is the genuinely independent one: a hand-maintained
	//     capability -> paths map that /health reports to operators. A route de-declared in the
	//     catalogue disappears from 1 and 2 together and is still listed here, so it reds by
	//     name. That was the measured evasion — setting two record routes to CapNone and
	//     reverting their handler guards left this guard green on floors 1 and 2 alone.
	//
	// The cross-check is ONE-DIRECTIONAL by design: it asks whether everything /health calls
	// gated is actually driven here. Deleting a route from BOTH lists still passes, because
	// then no source claims it is gated — that is a decision to stop gating a route, not a
	// drift between two records of the same decision, and it is not this guard's question.
	if len(gated) < 20 {
		t.Fatalf("only %d capability-gated routes in the catalogue; the enumeration has stopped matching and this guard would pass over almost nothing", len(gated))
	}
	for capability := range routes.CapabilityEndpoints() {
		if !seen[capability] {
			t.Errorf("capability %q gates endpoints but no catalogue route carries it, so nothing here drives it", capability)
		}
	}
	assertHealthGatedRoutesArePresent(t, gated)
	return gated
}

// assertHealthGatedRoutesArePresent cross-checks the enumerated population against the routes
// /health tells operators are gated. The two lists are maintained independently — one is the
// catalogue's Capability field, the other a literal map in security.go — so a route that loses
// its declaration in one is still named by the other.
//
// Only ROOT forms are compared: the health map also lists /tabs/{id}/ and /instances/ spellings,
// which this guard drives through their root form or not at all. The pairing is by SETTING, not
// by the map's key names, so an entry with no catalogue capability behind it — clipboard, whose
// gate is local — excludes itself rather than needing an exemption.
func assertHealthGatedRoutesArePresent(t *testing.T, gated []routes.Endpoint) {
	t.Helper()

	driven := map[string]bool{}
	for _, ep := range gated {
		driven[ep.Route()] = true
	}

	settingCapability := map[string]routes.Capability{}
	for capability := range routes.CapabilityEndpoints() {
		if meta, ok := routes.Meta(capability); ok && meta.Setting != "" {
			settingCapability[meta.Setting] = capability
		}
	}
	if len(settingCapability) == 0 {
		t.Fatal("no capability reports a setting, so this cross-check would compare against nothing")
	}

	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)
	checked := 0
	for name, state := range h.endpointSecurityStates() {
		if _, ok := settingCapability[state.Setting]; !ok {
			continue
		}
		for _, path := range state.Paths {
			if strings.Contains(path, "/tabs/{id}/") || strings.Contains(path, "/instances/") {
				continue
			}
			checked++
			if !driven[path] {
				t.Errorf("/health reports %q as gated under %q, but the catalogue no longer declares a capability for it, so this guard does not drive it — re-declare the route's capability, or remove it from endpointSecurityStates if it genuinely stopped being gated", path, name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no root-form path was cross-checked; the health map or its path spellings changed and this floor would pass over nothing")
	}
}

// driveBridgeRoute issues one request through the REAL bridge mux and returns what a client
// would receive. answered=false means the handler ran past the gate and then panicked on the
// mock bridge, so there is no response to judge — but getting that far is itself proof it was
// not refused, which is why the negative control treats it as a pass and the disabled run does
// not: there the recorder is still at 200 and the row fails on the status.
//
// The request runs under a deadline because an unguarded handler must FAIL this guard rather
// than hang it: with no browser behind the mock, a route that blocks would otherwise stall
// the suite instead of reporting the missing gate.
func driveBridgeRoute(t *testing.T, cfg *config.RuntimeConfig, ep routes.Endpoint) (int, string, bool) {
	t.Helper()

	h := New(&mockBridge{}, cfg, nil, nil, nil)
	mux := http.NewServeMux()
	h.registerBridgeRoutes(mux)

	method, path, _ := strings.Cut(ep.Route(), " ")
	w := httptest.NewRecorder()
	answered := true
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recover() != nil {
				answered = false
			}
		}()
		mux.ServeHTTP(w, httptest.NewRequest(method, concreteRoutePath(path), strings.NewReader("{}")))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not answer within the deadline; a capability gate answers before any browser work, so this route is reaching for one", ep.Route())
	}
	return w.Code, w.Body.String(), answered
}

// concreteRoutePath substitutes a value for each wildcard segment, so the guard sends what a
// client sends rather than a pattern that only matches by accident.
func concreteRoutePath(pattern string) string {
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[i] = "probe"
		}
	}
	return strings.Join(segments, "/")
}

// The invariant this file already pins for route EXISTENCE, one catalogue field along.
// registerBridgeRoutes walks routes.Core() and drops ep.Capability, so on the bridge front a
// capability gate is whatever each handler happens to do — and the same seam produced the same
// class of defect twice, most recently two of three sibling record routes serving with
// security.allowScreencast off while the third refused. Enforcement stays per-handler by
// decision; this is the mechanism that notices when one of them ships without its guard.
func TestEveryCapabilityGatedRouteRefusesOnTheBridgeFrontWhenDisabled(t *testing.T) {
	for _, ep := range gatedCatalogRoutes(t) {
		t.Run(ep.Route(), func(t *testing.T) {
			meta, ok := routes.Meta(ep.Capability)
			if !ok {
				t.Fatalf("capability %q has no metadata, so no refusal can name its setting", ep.Capability)
			}

			status, body, _ := driveBridgeRoute(t, &config.RuntimeConfig{}, ep)
			if status != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 with %s off; the bridge front serves a route the catalogue declares gated (body=%s)",
					status, meta.Setting, body)
			}

			var resp struct {
				Code    string         `json:"code"`
				Details map[string]any `json:"details"`
			}
			if err := json.Unmarshal([]byte(body), &resp); err != nil {
				t.Fatalf("refusal is not the product's error envelope (%v): %s", err, body)
			}
			// Compared against the catalogue's own metadata rather than a literal, so the
			// two fronts cannot answer one capability under two codes.
			if resp.Code != meta.DisabledCode {
				t.Errorf("code = %q, want %q", resp.Code, meta.DisabledCode)
			}
			if setting, _ := resp.Details["setting"].(string); setting != meta.Setting {
				t.Errorf("details.setting = %q, want %q", setting, meta.Setting)
			}
		})
	}
}

// The negative control: the gate has to be a gate, not a removal. With the capability on, no
// route in the same population answers the capability refusal. A handler that panics on the
// mock bridge got past the gate, which is the whole claim here, so it has nothing to judge and
// is not a failure.
func TestNoCapabilityGatedRouteRefusesWhenTheCapabilityIsEnabled(t *testing.T) {
	enabled := &config.RuntimeConfig{
		AllowEvaluate:         true,
		AllowMacro:            true,
		AllowScreencast:       true,
		AllowDownload:         true,
		AllowCookies:          true,
		AllowNetworkIntercept: true,
		AllowUpload:           true,
		AllowStateExport:      true,
	}

	for _, ep := range gatedCatalogRoutes(t) {
		t.Run(ep.Route(), func(t *testing.T) {
			meta, _ := routes.Meta(ep.Capability)
			status, body, answered := driveBridgeRoute(t, enabled, ep)
			if !answered {
				return
			}
			if status == http.StatusForbidden && strings.Contains(body, meta.DisabledCode) {
				t.Fatalf("%s still answers %s with %s enabled, so the gate cannot be opened: %s",
					ep.Route(), meta.DisabledCode, meta.Setting, body)
			}
		})
	}
}
