package orchestrator

import (
	"testing"

	"github.com/pinchtab/pinchtab/internal/session"
)

func TestSessionLifecycleHook_ClearsBinding(t *testing.T) {
	o := NewOrchestrator(t.TempDir())
	o.bindings.BindSession("ses_dead", "inst_a")

	hook := o.SessionLifecycleHook()
	hook(session.LifecycleEvent{SessionID: "ses_dead", AgentID: "agent-x", Reason: session.LifecycleReasonRevoked})

	if _, ok := o.bindings.ResolveSession("ses_dead"); ok {
		t.Fatal("binding should have been cleared")
	}
}

func TestSessionLifecycleHook_NoopOnEmptyID(t *testing.T) {
	o := NewOrchestrator(t.TempDir())
	o.bindings.BindSession("ses_a", "inst_a")
	hook := o.SessionLifecycleHook()
	hook(session.LifecycleEvent{SessionID: "", Reason: session.LifecycleReasonExpired})
	if _, ok := o.bindings.ResolveSession("ses_a"); !ok {
		t.Fatal("unrelated binding should not be touched")
	}
}
