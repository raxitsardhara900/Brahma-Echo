package browserprobe

import (
	"context"
	"sync"
)

// VersionCache probes a browser binary for its real version at most once per
// resolved binary path. RunVersion spawns a process, so callers on request
// paths must go through a cache rather than probing per call.
//
// The probe function is a constructor parameter rather than a package variable
// so a test can count probes and serve two fake binaries different versions
// without global state.
type VersionCache struct {
	probe func(context.Context, string) (string, error)

	mu      sync.Mutex
	entries map[string]string
	probes  int
}

func NewVersionCache(probe func(context.Context, string) (string, error)) *VersionCache {
	if probe == nil {
		probe = RunVersion
	}
	return &VersionCache{probe: probe, entries: map[string]string{}}
}

// Version returns the version token the binary reports, or "" when the binary
// is unknown or the probe fails. A failure is cached as "" so a missing or
// unrunnable binary cannot spawn a process on every request.
func (c *VersionCache) Version(ctx context.Context, binary string) string {
	if binary == "" {
		return ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if version, ok := c.entries[binary]; ok {
		return version
	}

	c.probes++
	line, err := c.probe(ctx, binary)
	version := ""
	if err == nil {
		version = ExtractVersionToken(line)
	}
	c.entries[binary] = version
	return version
}

// Probes reports how many times the probe function actually ran, so a test can
// assert the cache serves repeats without spawning another process.
func (c *VersionCache) Probes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.probes
}
