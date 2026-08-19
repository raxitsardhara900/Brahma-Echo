package server

import (
	"testing"

	"github.com/pinchtab/pinchtab/internal/srccensus"
)

// The registrars are unit-tested against a bare mux, but that cannot show either MODE
// calls one — and a mode that stops calling it is silently back to the bare mux 404 this
// card exists to remove. Neither entry point is drivable from a unit test: RunBridgeServer
// and RunDashboard bind a listener, launch a browser and block on signals.
//
// So the wiring is pinned at the source, inside the enclosing function rather than merely
// somewhere in the package: a call that drifted out of the mode's own body would otherwise
// still satisfy it.
func TestBothModesWireTheSessionFamilyToACodedRefusal(t *testing.T) {
	pkg := srccensus.Load(t, ".", 4)

	for _, wiring := range []struct {
		mode      string
		registrar string
		why       string
	}{
		{"RunBridgeServer", "RegisterSessionsUnavailableInBridgeMode",
			"a bridge never mounts the family, so without this POST /sessions is a bare 404 and the CLI cannot tell it from a typo"},
		{"RunDashboard", "RegisterSessionsDisabled",
			"a full server with sessions.agent.enabled false does not mount the family either, and that is the one state whose remedy is the config edit"},
	} {
		t.Run(wiring.mode, func(t *testing.T) {
			fn, ok := pkg.Func(wiring.mode)
			if !ok {
				t.Fatalf("%s not found in %s; if it was renamed, re-point this guard at the new entry point rather than deleting it", wiring.mode, pkg.Dir())
			}

			inside := 0
			for _, site := range pkg.Calls(t, wiring.registrar) {
				if pkg.Contains(fn, site) {
					inside++
				}
			}
			if inside == 0 {
				t.Errorf("%s never calls %s, so that mode answers the session family with net/http's bare 404: %s",
					wiring.mode, wiring.registrar, wiring.why)
			}
		})
	}
}
