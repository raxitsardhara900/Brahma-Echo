package dashboard

import (
	"net/http"
	"testing"

	"github.com/pinchtab/pinchtab/internal/session"
)

// This is the enforcement criterion 10 asks about, and it is what stops the defect
// regenerating one route at a time. The live registration walks session.RoutePatterns()
// and panics on a pattern it cannot bind, so a route added to the shared list without a
// handler fails loudly here instead of being mounted in server mode and answering a bare
// mux 404 in the other two.
//
// Driving handlerFor over the real list is the browserless half: it needs no store and no
// server, so it runs everywhere and names the unbound pattern.
func TestEverySharedSessionRouteHasAHandler(t *testing.T) {
	patterns := session.RoutePatterns()
	if len(patterns) == 0 {
		t.Fatal("the shared route list is empty, so this census would pass over nothing")
	}

	api := NewSessionAPI(nil, nil)
	for _, pattern := range patterns {
		if api.handlerFor(pattern) == nil {
			t.Errorf("%s is in session.RoutePatterns() but binds no handler; RegisterHandlers panics on it, and the unavailable-mode registrars would answer it while server mode 404s", pattern)
		}
	}
}

// The reverse direction: a handler this package can serve but which is NOT in the shared
// list would be mounted in server mode only, which is the asymmetry the list exists to
// prevent. Enumerating the patterns the switch accepts is the only way to see it.
func TestNoSessionHandlerIsReachableOutsideTheSharedList(t *testing.T) {
	// The switch's own arms, spelled once here. A pattern added to handlerFor without
	// being added to the shared list fails this.
	served := []string{
		"POST /sessions",
		"GET /sessions",
		"GET /sessions/me",
		"GET /sessions/{id}",
		"POST /sessions/{id}/revoke",
	}

	shared := map[string]bool{}
	for _, pattern := range session.RoutePatterns() {
		shared[pattern] = true
	}

	api := NewSessionAPI(nil, nil)
	for _, pattern := range served {
		if api.handlerFor(pattern) == nil {
			t.Errorf("%s no longer binds a handler; if the route was removed, remove it from this list and from session.RoutePatterns()", pattern)
			continue
		}
		if !shared[pattern] {
			t.Errorf("%s binds a handler but is absent from session.RoutePatterns(), so it is mounted in server mode and a bare 404 in bridge mode", pattern)
		}
	}
	if len(served) != len(shared) {
		t.Errorf("handlerFor serves %d patterns but the shared list holds %d; the two definitions have diverged", len(served), len(shared))
	}
}

// RegisterHandlers must refuse to leave a listed route unrouted rather than skipping it,
// because a silently skipped pattern is exactly the bare-404 state.
func TestRegisterHandlersPanicsOnAListedRouteItCannotBind(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an unbindable pattern was skipped silently; it would answer a bare 404 while its siblings work")
		}
	}()

	api := NewSessionAPI(newTestSessionStore(), nil)
	mux := http.NewServeMux()
	// Stand in for a route added to the shared list with no handler.
	api.registerPatterns(mux, []string{"GET /sessions/unbound"})
}
