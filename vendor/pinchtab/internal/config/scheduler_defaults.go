package config

// DefaultSchedulerConfig is the one place the scheduler's tuning defaults are
// written. internal/scheduler imports this package, so the values live here and
// travel to their consumer rather than the other way round — which also makes them
// visible to `config get`, since the effective-value reader loads a RuntimeConfig.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		Enabled:           false,
		Strategy:          "fair-fifo",
		MaxQueueSize:      1000,
		MaxPerAgent:       100,
		MaxInflight:       20,
		MaxPerAgentFlight: 10,
		ResultTTLSec:      300,
		WorkerCount:       4,
		MaxBatchSize:      50,
	}
}

// finalizeSchedulerConfig restores the default for every knob left at or below zero,
// which is what a configured 0 has always meant here: use the default, never
// unlimited.
func finalizeSchedulerConfig(cfg *RuntimeConfig) {
	defaults := DefaultSchedulerConfig()
	s := &cfg.Scheduler
	if s.Strategy == "" {
		s.Strategy = defaults.Strategy
	}
	for _, knob := range []struct {
		value    *int
		fallback int
	}{
		{&s.MaxQueueSize, defaults.MaxQueueSize},
		{&s.MaxPerAgent, defaults.MaxPerAgent},
		{&s.MaxInflight, defaults.MaxInflight},
		{&s.MaxPerAgentFlight, defaults.MaxPerAgentFlight},
		{&s.ResultTTLSec, defaults.ResultTTLSec},
		{&s.WorkerCount, defaults.WorkerCount},
		{&s.MaxBatchSize, defaults.MaxBatchSize},
	} {
		if *knob.value <= 0 {
			*knob.value = knob.fallback
		}
	}
}
