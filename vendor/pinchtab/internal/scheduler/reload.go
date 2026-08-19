package scheduler

import (
	"log/slog"
	"sync"
	"time"
)

// ReloadConfig applies new queue limits without restarting the scheduler.
// Only tuning knobs that are safe to change at runtime are updated.
func (s *Scheduler) ReloadConfig(cfg Config) {
	if cfg.MaxQueueSize > 0 {
		s.queue.SetLimits(cfg.MaxQueueSize, cfg.MaxPerAgent)
		slog.Info("scheduler: queue limits reloaded",
			"maxQueueSize", cfg.MaxQueueSize,
			"maxPerAgent", cfg.MaxPerAgent,
		)
	}
	if cfg.MaxInflight > 0 {
		s.cfgMu.Lock()
		s.cfg.MaxInflight = cfg.MaxInflight
		s.cfg.MaxPerAgentFlight = cfg.MaxPerAgentFlight
		s.cfgMu.Unlock()
		// A raised cap can make queued tasks eligible; workers parked on Ready
		// have no poll fallback and would otherwise wait for unrelated traffic.
		s.queue.signal()
		slog.Info("scheduler: inflight limits reloaded",
			"maxInflight", cfg.MaxInflight,
			"maxPerAgentFlight", cfg.MaxPerAgentFlight,
		)
	}
	if cfg.ResultTTL > 0 {
		s.results.SetTTL(cfg.ResultTTL)
		slog.Info("scheduler: result TTL reloaded", "ttl", cfg.ResultTTL)
	}
}

// ConfigWatcher periodically re-reads the scheduler config and applies changes.
type ConfigWatcher struct {
	interval time.Duration
	loadFn   func() (Config, error)
	sched    *Scheduler
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewConfigWatcher creates a watcher that calls loadFn at the given interval
// and applies any changed values to the scheduler.
func NewConfigWatcher(interval time.Duration, loadFn func() (Config, error), sched *Scheduler) *ConfigWatcher {
	return &ConfigWatcher{
		interval: interval,
		loadFn:   loadFn,
		sched:    sched,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the config watch loop.
func (cw *ConfigWatcher) Start() {
	cw.wg.Add(1)
	go cw.run()
}

// Stop terminates the config watcher.
func (cw *ConfigWatcher) Stop() {
	close(cw.stopCh)
	cw.wg.Wait()
}

func (cw *ConfigWatcher) run() {
	defer cw.wg.Done()
	ticker := time.NewTicker(cw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-cw.stopCh:
			return
		case <-ticker.C:
			cfg, err := cw.loadFn()
			if err != nil {
				slog.Warn("config watcher: failed to load config", "err", err)
				continue
			}
			cw.sched.ReloadConfig(cfg)
		}
	}
}
