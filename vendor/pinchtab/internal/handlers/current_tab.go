package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pinchtab/pinchtab/internal/activity"
	"github.com/pinchtab/pinchtab/internal/session"
)

const (
	currentTabScopeGlobal  = "global"
	currentTabScopeSession = "session"
	currentTabScopeAgent   = "agent"
)

type currentTabScope struct {
	kind  string
	key   string
	label string
}

func (s currentTabScope) IsGlobal() bool {
	return s.kind == currentTabScopeGlobal
}

func (s currentTabScope) Description() string {
	if s.label != "" {
		return s.label
	}
	return "global"
}

// defaultCurrentTabCap caps the number of session/agent → tab entries
// the instance keeps in memory. When new entries push past the cap, the
// least-recently-touched one is evicted. The cap exists because we no
// longer push session-revoke notices over the wire — entries for dead
// sessions accumulate until pushed out by normal traffic, and the cap
// keeps that bounded.
const defaultCurrentTabCap = 5000

// currentTabEntry holds a tab id plus a per-entry timestamp used as the
// LRU recency signal. lastTouched is updated on Get, Set, and any access
// that proves the entry is still in active use.
type currentTabEntry struct {
	tabID       string
	lastTouched time.Time
}

// CurrentTabStore tracks server-side current-tab pointers for identified
// automation callers. The anonymous/global current tab remains owned by the
// bridge's existing current-tab behavior for backward compatibility.
type CurrentTabStore struct {
	mu      sync.RWMutex
	entries map[string]currentTabEntry
	cap     int
	now     func() time.Time
}

func NewCurrentTabStore() *CurrentTabStore {
	return &CurrentTabStore{
		entries: make(map[string]currentTabEntry),
		cap:     defaultCurrentTabCap,
		now:     time.Now,
	}
}

// SetCap overrides the eviction cap (mainly for tests). A cap of 0 or
// less leaves the existing cap unchanged.
func (s *CurrentTabStore) SetCap(n int) {
	if s == nil || n <= 0 {
		return
	}
	s.mu.Lock()
	s.cap = n
	s.evictExcessLocked()
	s.mu.Unlock()
}

func (s *CurrentTabStore) Get(scope currentTabScope) (string, bool) {
	if s == nil || scope.IsGlobal() || scope.key == "" {
		return "", false
	}
	s.mu.Lock()
	entry, ok := s.entries[scope.key]
	if !ok || entry.tabID == "" {
		s.mu.Unlock()
		return "", false
	}
	entry.lastTouched = s.now()
	s.entries[scope.key] = entry
	s.mu.Unlock()
	return entry.tabID, true
}

func (s *CurrentTabStore) Set(scope currentTabScope, tabID string) {
	if s == nil || scope.IsGlobal() || scope.key == "" {
		return
	}
	tabID = strings.TrimSpace(tabID)
	if tabID == "" {
		return
	}
	s.mu.Lock()
	s.entries[scope.key] = currentTabEntry{tabID: tabID, lastTouched: s.now()}
	s.evictExcessLocked()
	s.mu.Unlock()
}

// evictExcessLocked drops least-recently-touched entries until the map
// fits within s.cap. Caller must hold s.mu (write).
func (s *CurrentTabStore) evictExcessLocked() {
	if s.cap <= 0 || len(s.entries) <= s.cap {
		return
	}
	// Single-pass scan to find the oldest entry, drop, repeat. Cheap
	// because evictions are rare (only fire on Set when at the cap).
	for len(s.entries) > s.cap {
		var (
			oldestKey string
			oldestAt  time.Time
			first     = true
		)
		for k, e := range s.entries {
			if first || e.lastTouched.Before(oldestAt) {
				oldestKey = k
				oldestAt = e.lastTouched
				first = false
			}
		}
		delete(s.entries, oldestKey)
	}
}

func (s *CurrentTabStore) Clear(scope currentTabScope) {
	if s == nil || scope.IsGlobal() || scope.key == "" {
		return
	}
	s.mu.Lock()
	delete(s.entries, scope.key)
	s.mu.Unlock()
}

func (s *CurrentTabStore) ClearTab(tabID string) {
	if s == nil {
		return
	}
	tabID = strings.TrimSpace(tabID)
	if tabID == "" {
		return
	}
	s.mu.Lock()
	for key, entry := range s.entries {
		if entry.tabID == tabID {
			delete(s.entries, key)
		}
	}
	s.mu.Unlock()
}

func currentTabScopeFromRequest(r *http.Request) currentTabScope {
	if sess, ok := session.FromRequest(r); ok && sess != nil {
		if id := strings.TrimSpace(sess.ID); id != "" {
			return scopedCurrentTab(currentTabScopeSession, id)
		}
	}
	if r != nil {
		// X-PinchTab-Session-Id is internal metadata; honor it only on
		// trusted-internal-proxy hops (orchestrator → instance) where the
		// strip middleware preserved the header. Public requests have it
		// stripped at ingress, but defense in depth.
		if IsTrustedInternalProxy(r) {
			if id := strings.TrimSpace(r.Header.Get(activity.HeaderPTSessionID)); id != "" {
				return scopedCurrentTab(currentTabScopeSession, id)
			}
		}
		if id := strings.TrimSpace(r.Header.Get(activity.HeaderAgentID)); id != "" {
			return scopedCurrentTab(currentTabScopeAgent, id)
		}
	}
	return currentTabScope{kind: currentTabScopeGlobal, key: currentTabScopeGlobal, label: "global"}
}

func scopedCurrentTab(kind, id string) currentTabScope {
	return currentTabScope{
		kind:  kind,
		key:   kind + ":" + id,
		label: fmt.Sprintf("%s %q", kind, id),
	}
}
