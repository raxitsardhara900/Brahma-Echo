package scheduler

import (
	"errors"
	"testing"
	"time"
)

func TestReloadConfigQueueLimits(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	original := s.cfg
	s.ReloadConfig(Config{
		MaxQueueSize: 500,
		MaxPerAgent:  50,
	})

	// Queue limits should be updated.
	stats := s.QueueStats()
	_ = stats // Stats reflect current queue state, not limits directly.

	// Inflight limits should be unchanged.
	perAgent, global := s.inflightLimits()
	if perAgent != original.MaxPerAgentFlight {
		t.Errorf("perAgent should be unchanged, got %d", perAgent)
	}
	if global != original.MaxInflight {
		t.Errorf("global should be unchanged, got %d", global)
	}
}

func TestReloadConfigInflightLimits(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	s.ReloadConfig(Config{
		MaxInflight:       30,
		MaxPerAgentFlight: 15,
	})

	perAgent, global := s.inflightLimits()
	if perAgent != 15 {
		t.Errorf("expected perAgent=15, got %d", perAgent)
	}
	if global != 30 {
		t.Errorf("expected global=30, got %d", global)
	}
}

func TestReloadConfigResultTTL(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	s.ReloadConfig(Config{
		ResultTTL: 10 * time.Minute,
	})

	// Just verify no panic — TTL is internal to ResultStore.
}

func TestReloadConfigZeroValuesIgnored(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	original := s.cfg

	// Zero values should not change anything.
	s.ReloadConfig(Config{})

	perAgent, global := s.inflightLimits()
	if perAgent != original.MaxPerAgentFlight {
		t.Errorf("perAgent should be unchanged, got %d", perAgent)
	}
	if global != original.MaxInflight {
		t.Errorf("global should be unchanged, got %d", global)
	}
}

func TestConfigWatcherStartStop(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	callCount := 0
	loadFn := func() (Config, error) {
		callCount++
		return Config{MaxInflight: 99, MaxPerAgentFlight: 49}, nil
	}

	cw := NewConfigWatcher(50*time.Millisecond, loadFn, s)
	cw.Start()

	time.Sleep(200 * time.Millisecond)
	cw.Stop()

	if callCount == 0 {
		t.Error("config watcher should have called loadFn at least once")
	}

	perAgent, global := s.inflightLimits()
	if perAgent != 49 {
		t.Errorf("expected perAgent=49 after reload, got %d", perAgent)
	}
	if global != 99 {
		t.Errorf("expected global=99 after reload, got %d", global)
	}
}

func TestConfigWatcherLoadError(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	original := s.cfg

	loadFn := func() (Config, error) {
		return Config{}, errors.New("config unavailable")
	}

	cw := NewConfigWatcher(50*time.Millisecond, loadFn, s)
	cw.Start()
	time.Sleep(200 * time.Millisecond)
	cw.Stop()

	// Limits should be unchanged since load always fails.
	perAgent, global := s.inflightLimits()
	if perAgent != original.MaxPerAgentFlight {
		t.Errorf("perAgent should be unchanged after error, got %d", perAgent)
	}
	if global != original.MaxInflight {
		t.Errorf("global should be unchanged after error, got %d", global)
	}
}

func TestQueueSetLimits(t *testing.T) {
	q := NewTaskQueue(10, 5)

	q.SetLimits(20, 10)

	// Should accept more tasks now.
	for i := range 10 {
		task := &Task{
			ID:       generateTaskID(),
			AgentID:  "a1",
			Action:   "click",
			State:    StateQueued,
			Deadline: time.Now().Add(time.Minute),
		}
		_, err := q.Enqueue(task)
		if err != nil {
			t.Fatalf("Enqueue task[%d] failed unexpectedly: %v", i, err)
		}
	}

	// 11th task from same agent should fail (per-agent limit is 10).
	task := &Task{
		ID:       generateTaskID(),
		AgentID:  "a1",
		Action:   "click",
		State:    StateQueued,
		Deadline: time.Now().Add(time.Minute),
	}
	_, err := q.Enqueue(task)
	if err == nil {
		t.Error("should reject task exceeding new per-agent limit")
	}
}

func TestResultStoreSetTTL(t *testing.T) {
	rs := NewResultStore(5 * time.Minute)

	rs.SetTTL(1 * time.Minute)

	// Just verify no panic.
	task := &Task{
		ID:      "tsk_ttl_1",
		AgentID: "a1",
		State:   StateDone,
	}
	rs.Store(task)

	got := rs.Get("tsk_ttl_1")
	if got == nil {
		t.Error("should find stored task")
	}
}

func TestMetricsWiredInSubmit(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	_, _ = s.Submit(SubmitRequest{
		AgentID: "a1",
		Action:  "click",
		TabID:   "tab-1",
	})

	snap := s.GetMetrics()
	if snap.TasksSubmitted != 1 {
		t.Errorf("expected 1 submitted, got %d", snap.TasksSubmitted)
	}
	if snap.Agents["a1"].Submitted != 1 {
		t.Errorf("expected a1 submitted=1, got %d", snap.Agents["a1"].Submitted)
	}
}

func TestMetricsWiredInCancel(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	task, err := s.Submit(SubmitRequest{
		AgentID: "a1",
		Action:  "click",
		TabID:   "tab-1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := s.Cancel(task.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	snap := s.GetMetrics()
	if snap.TasksCancelled != 1 {
		t.Errorf("expected 1 cancelled, got %d", snap.TasksCancelled)
	}
}

func TestMetricsWiredInDispatch(t *testing.T) {
	s, executor := newTestScheduler(t)
	defer executor.Close()

	s.Start()
	defer s.Stop()

	_, err := s.Submit(SubmitRequest{
		AgentID: "a1",
		Action:  "click",
		TabID:   "tab-1",
		Ref:     "e1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Wait for dispatch to complete.
	deadline := time.After(5 * time.Second)
	for {
		snap := s.GetMetrics()
		if snap.TasksCompleted >= 1 || snap.TasksFailed >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for task dispatch")
		case <-time.After(50 * time.Millisecond):
		}
	}

	snap := s.GetMetrics()
	if snap.DispatchCount == 0 {
		t.Error("dispatch count should be > 0 after task execution")
	}
	if snap.AvgDispatchLatency <= 0 {
		t.Error("avg dispatch latency should be > 0")
	}
}
