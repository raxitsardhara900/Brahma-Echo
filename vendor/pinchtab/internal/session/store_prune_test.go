package session

import (
	"sync"
	"testing"
	"time"
)

// A session that is simply abandoned is never re-authenticated, so the lazy
// expiry checks in authenticate/Touch never run for it. PruneExpired is the
// only thing that evicts it and tells downstream binding tables to drop it.
func TestPruneExpiredEvictsAbandonedSessionAndNotifies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewStore(Config{Enabled: true, IdleTimeout: time.Hour, MaxLifetime: 24 * time.Hour})
	s.now = func() time.Time { return now }

	var (
		mu     sync.Mutex
		events []LifecycleEvent
	)
	s.OnLifecycle(func(evt LifecycleEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})

	id, _, err := s.Create("agent-1", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Nothing touches the session again; it just ages past the idle timeout.
	now = now.Add(2 * time.Hour)
	s.PruneExpired()

	if _, ok := s.Get(id); ok {
		t.Error("abandoned session survived PruneExpired")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		got := len(events)
		mu.Unlock()
		if got > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Fatalf("got %d lifecycle events, want 1", len(events))
	}
	if events[0].SessionID != id || events[0].Reason != LifecycleReasonPruned {
		t.Errorf("event = %+v, want pruned event for %s", events[0], id)
	}
}

// A live session must survive the sweep, or every maintenance tick would log
// active agents out.
func TestPruneExpiredKeepsLiveSessions(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := NewStore(Config{Enabled: true, IdleTimeout: time.Hour, MaxLifetime: 24 * time.Hour})
	s.now = func() time.Time { return now }

	id, token, err := s.Create("agent-1", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	now = now.Add(30 * time.Minute)
	s.PruneExpired()

	if _, ok := s.Get(id); !ok {
		t.Fatal("live session was pruned")
	}
	if _, ok := s.Authenticate(token); !ok {
		t.Error("live session no longer authenticates after a sweep")
	}
}
