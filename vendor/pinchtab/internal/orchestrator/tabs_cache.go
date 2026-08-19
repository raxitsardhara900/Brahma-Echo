package orchestrator

import (
	"sync"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
)

// tabsCacheTTL is the default freshness window for cached per-instance tab
// snapshots. Short enough that a stale read at 1Hz dashboard polling
// rarely outlives a single tick, long enough to absorb bursts.
const tabsCacheTTL = 1500 * time.Millisecond

// tabsSnapshot is a per-instance tab list with its expiry deadline.
type tabsSnapshot struct {
	tabs      []bridge.InstanceTab
	expiresAt time.Time
}

// TabsCache stores per-instance snapshots of /tabs results for the
// dashboard's aggregate endpoints. Routing decisions never read this cache
// — its only job is to absorb repeated visibility queries.
//
// Invalidation hooks bump entries on any successful proxied response that
// could affect the tab list (open / close / navigate / reload / history).
type TabsCache struct {
	mu   sync.RWMutex
	ttl  time.Duration
	now  func() time.Time
	data map[string]tabsSnapshot
}

// NewTabsCache returns an empty cache. Pass zero ttl to use tabsCacheTTL.
// Pass nil now to use time.Now (tests can inject a deterministic clock).
func NewTabsCache(ttl time.Duration, now func() time.Time) *TabsCache {
	if ttl <= 0 {
		ttl = tabsCacheTTL
	}
	if now == nil {
		now = time.Now
	}
	return &TabsCache{
		ttl:  ttl,
		now:  now,
		data: make(map[string]tabsSnapshot),
	}
}

// Get returns the cached snapshot for instanceID if it has not expired.
// The returned slice is the cache's storage; callers that mutate it must
// copy first.
func (c *TabsCache) Get(instanceID string) ([]bridge.InstanceTab, bool) {
	if c == nil || instanceID == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap, ok := c.data[instanceID]
	if !ok || c.now().After(snap.expiresAt) {
		return nil, false
	}
	return snap.tabs, true
}

// Set stores a fresh snapshot for instanceID.
func (c *TabsCache) Set(instanceID string, tabs []bridge.InstanceTab) {
	if c == nil || instanceID == "" {
		return
	}
	stored := make([]bridge.InstanceTab, len(tabs))
	copy(stored, tabs)
	c.mu.Lock()
	c.data[instanceID] = tabsSnapshot{
		tabs:      stored,
		expiresAt: c.now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Invalidate drops the snapshot for instanceID. No-op if absent.
func (c *TabsCache) Invalidate(instanceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.mu.Lock()
	delete(c.data, instanceID)
	c.mu.Unlock()
}

// InvalidateAll clears every entry. Used on broad lifecycle events
// (e.g. instance restart) where targeted invalidation is impractical.
func (c *TabsCache) InvalidateAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.data = make(map[string]tabsSnapshot)
	c.mu.Unlock()
}

// Len returns the current number of cached instances. Useful for metrics
// and tests.
func (c *TabsCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}
