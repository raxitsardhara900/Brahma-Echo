package scheduler

import "testing"

func drainReady(q *TaskQueue) {
	select {
	case <-q.Ready():
	default:
	}
}

// Workers block on Ready() with no poll fallback, so every state change that
// can newly permit a Dequeue must signal. Completing a task frees a global
// inflight slot that another agent's queued task needs.
func TestCompleteSignalsWhenOtherAgentHasQueuedWork(t *testing.T) {
	q := NewTaskQueue(10, 10)

	if _, err := q.Enqueue(&Task{ID: "a1", AgentID: "A"}); err != nil {
		t.Fatalf("enqueue a1: %v", err)
	}
	drainReady(q)

	running := q.Dequeue(5, 1)
	if running == nil {
		t.Fatal("a1 should dequeue")
	}

	if _, err := q.Enqueue(&Task{ID: "b1", AgentID: "B"}); err != nil {
		t.Fatalf("enqueue b1: %v", err)
	}
	drainReady(q)

	if blocked := q.Dequeue(5, 1); blocked != nil {
		t.Fatalf("b1 must be blocked by the global inflight cap, got %s", blocked.ID)
	}

	q.Complete(running.AgentID)

	select {
	case <-q.Ready():
	default:
		t.Fatal("Complete freed the only global inflight slot but did not signal; b1 stalls until an unrelated Enqueue")
	}
	if got := q.Dequeue(5, 1); got == nil || got.ID != "b1" {
		t.Fatalf("b1 should dequeue after the slot frees, got %v", got)
	}
}

// notify holds a single pending signal, so a burst of enqueues wakes only one
// worker. A successful Dequeue that leaves work behind must hand the wake-up on.
func TestDequeueCascadesWakeupWhenWorkRemains(t *testing.T) {
	q := NewTaskQueue(10, 10)

	for _, id := range []string{"a1", "a2", "a3"} {
		if _, err := q.Enqueue(&Task{ID: id, AgentID: "A"}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	drainReady(q)

	if got := q.Dequeue(5, 5); got == nil {
		t.Fatal("first task should dequeue")
	}

	select {
	case <-q.Ready():
	default:
		t.Fatal("Dequeue left queued work but did not signal; idle workers stay blocked")
	}
}
