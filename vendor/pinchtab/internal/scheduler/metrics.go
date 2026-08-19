package scheduler

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks scheduler-level counters and latency for observability.
type Metrics struct {
	TasksSubmitted  atomic.Uint64
	TasksCompleted  atomic.Uint64
	TasksFailed     atomic.Uint64
	TasksCancelled  atomic.Uint64
	TasksRejected   atomic.Uint64
	TasksExpired    atomic.Uint64
	DispatchTotal   atomic.Uint64
	DispatchLatency atomic.Uint64 // cumulative milliseconds

	// Per-agent counters protected by mutex.
	mu         sync.RWMutex
	agentStats map[string]*agentMetricEntry
}

// AgentMetrics holds per-agent observability counters.
type AgentMetrics struct {
	Submitted uint64 `json:"submitted"`
	Completed uint64 `json:"completed"`
	Failed    uint64 `json:"failed"`
	Cancelled uint64 `json:"cancelled"`
	Rejected  uint64 `json:"rejected"`
}

func newMetrics() *Metrics {
	return &Metrics{
		agentStats: make(map[string]*agentMetricEntry),
	}
}

func (m *Metrics) recordSubmit(agentID string) {
	m.TasksSubmitted.Add(1)
	m.agentMetric(agentID).add(func(a *AgentMetrics) { a.Submitted++ })
}

func (m *Metrics) recordReject(agentID string) {
	m.TasksRejected.Add(1)
	m.agentMetric(agentID).add(func(a *AgentMetrics) { a.Rejected++ })
}

func (m *Metrics) recordComplete(agentID string) {
	m.TasksCompleted.Add(1)
	m.agentMetric(agentID).add(func(a *AgentMetrics) { a.Completed++ })
}

func (m *Metrics) recordFail(agentID string) {
	m.TasksFailed.Add(1)
	m.agentMetric(agentID).add(func(a *AgentMetrics) { a.Failed++ })
}

func (m *Metrics) recordCancel(agentID string) {
	m.TasksCancelled.Add(1)
	m.agentMetric(agentID).add(func(a *AgentMetrics) { a.Cancelled++ })
}

func (m *Metrics) recordExpire() {
	m.TasksExpired.Add(1)
}

func (m *Metrics) recordDispatchLatency(d time.Duration) {
	if d <= 0 {
		d = time.Nanosecond
	}
	m.DispatchTotal.Add(1)
	m.DispatchLatency.Add(uint64(d.Nanoseconds()))
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	dispatched := m.DispatchTotal.Load()
	latencyNs := m.DispatchLatency.Load()
	avgMs := 0.0
	if dispatched > 0 {
		avgMs = float64(latencyNs) / float64(dispatched) / 1e6
	}

	m.mu.RLock()
	agents := make(map[string]AgentMetrics, len(m.agentStats))
	for id, a := range m.agentStats {
		a.mu.RLock()
		agents[id] = AgentMetrics{
			Submitted: a.Submitted,
			Completed: a.Completed,
			Failed:    a.Failed,
			Cancelled: a.Cancelled,
			Rejected:  a.Rejected,
		}
		a.mu.RUnlock()
	}
	m.mu.RUnlock()

	return MetricsSnapshot{
		TasksSubmitted:     m.TasksSubmitted.Load(),
		TasksCompleted:     m.TasksCompleted.Load(),
		TasksFailed:        m.TasksFailed.Load(),
		TasksCancelled:     m.TasksCancelled.Load(),
		TasksRejected:      m.TasksRejected.Load(),
		TasksExpired:       m.TasksExpired.Load(),
		DispatchCount:      dispatched,
		AvgDispatchLatency: avgMs,
		Agents:             agents,
	}
}

// MetricsSnapshot is a serializable point-in-time view of scheduler metrics.
type MetricsSnapshot struct {
	TasksSubmitted     uint64                  `json:"tasksSubmitted"`
	TasksCompleted     uint64                  `json:"tasksCompleted"`
	TasksFailed        uint64                  `json:"tasksFailed"`
	TasksCancelled     uint64                  `json:"tasksCancelled"`
	TasksRejected      uint64                  `json:"tasksRejected"`
	TasksExpired       uint64                  `json:"tasksExpired"`
	DispatchCount      uint64                  `json:"dispatchCount"`
	AvgDispatchLatency float64                 `json:"avgDispatchLatencyMs"`
	Agents             map[string]AgentMetrics `json:"agents"`
}

// agentMetricEntry wraps AgentMetrics with its own lock for concurrent updates.
type agentMetricEntry struct {
	mu sync.RWMutex
	AgentMetrics
}

func (e *agentMetricEntry) add(fn func(*AgentMetrics)) {
	e.mu.Lock()
	fn(&e.AgentMetrics)
	e.mu.Unlock()
}

func (m *Metrics) agentMetric(agentID string) *agentMetricEntry {
	m.mu.RLock()
	if e, ok := m.agentStats[agentID]; ok {
		m.mu.RUnlock()
		return e
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after upgrading lock.
	if e, ok := m.agentStats[agentID]; ok {
		return e
	}
	e := &agentMetricEntry{}
	m.agentStats[agentID] = e
	return e
}
