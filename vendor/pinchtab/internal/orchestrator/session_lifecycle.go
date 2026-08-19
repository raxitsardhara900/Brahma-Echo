package orchestrator

import (
	"github.com/pinchtab/pinchtab/internal/session"
)

// SessionLifecycleHook returns a session.LifecycleHook the caller can wire
// into a session.Store via OnLifecycle. The hook drops the session →
// instance binding so a future request that reuses the same session id
// will be re-routed via the normal precedence rules instead of resurrecting
// the old binding.
//
// The instance-side scoped current-tab map is bounded (LRU cap + stale
// detection on the bridge), so we deliberately do NOT propagate eviction
// over the network. Dead entries on the instance get pushed out by normal
// traffic; this avoids a separate trusted internal HTTP surface.
func (o *Orchestrator) SessionLifecycleHook() session.LifecycleHook {
	if o == nil {
		return func(session.LifecycleEvent) {}
	}
	return func(evt session.LifecycleEvent) {
		if evt.SessionID == "" {
			return
		}
		o.bindings.ClearSession(evt.SessionID)
	}
}
