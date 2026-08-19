package dashboard

import (
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/activity"
)

// The dashboard identifies an activity event by its request id alone, but the
// activity store keys events on a composite of timestamp, source, request id,
// method, path, status and tab — because one request can emit several events.
// Where those two notions of identity disagree, the dashboard silently drops
// rows as duplicates.
func TestRecordActivityEventKeepsDistinctEventsSharingARequestID(t *testing.T) {
	d := NewDashboard(nil)

	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	events := []activity.Event{
		{RequestID: "req-1", Source: "client", Method: "POST", Path: "/action", TabID: "tab-a", Status: 200, Timestamp: base},
		{RequestID: "req-1", Source: "client", Method: "POST", Path: "/action", TabID: "tab-b", Status: 200, Timestamp: base.Add(time.Millisecond)},
	}
	for _, evt := range events {
		d.RecordActivityEvent(evt)
	}

	got := d.RecentEvents()
	if len(got) != 2 {
		t.Fatalf("recorded %d of 2 events sharing a request id: %+v", len(got), got)
	}

	tabs := map[string]bool{}
	for _, evt := range got {
		if tab, ok := evt.Details["tabId"].(string); ok {
			tabs[tab] = true
		}
	}
	if !tabs["tab-a"] || !tabs["tab-b"] {
		t.Errorf("both tabs should be represented, got %v", tabs)
	}
}

// Genuinely repeated ingestion of the same event must still collapse, or the
// re-sync poll would duplicate every row it re-reads.
func TestRecordActivityEventStillDedupesIdenticalEvents(t *testing.T) {
	d := NewDashboard(nil)

	evt := activity.Event{
		RequestID: "req-2", Source: "client", Method: "GET", Path: "/snapshot",
		TabID: "tab-a", Status: 200,
		Timestamp: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}
	d.RecordActivityEvent(evt)
	d.RecordActivityEvent(evt)

	if got := d.RecentEvents(); len(got) != 1 {
		t.Errorf("identical event ingested twice produced %d rows, want 1", len(got))
	}
}

// requestIDFor returns the X-Request-Id header or empty — it never generates
// one — so most events carry no request id at all. normalizeEvent then mints a
// fresh random id per ingest, defeating dedup entirely for those events.
func TestRecordActivityEventDedupesEventsWithoutARequestID(t *testing.T) {
	d := NewDashboard(nil)

	evt := activity.Event{
		Source: "client", Method: "GET", Path: "/snapshot", TabID: "tab-a", Status: 200,
		Timestamp: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}
	d.RecordActivityEvent(evt)
	d.RecordActivityEvent(evt)

	if got := d.RecentEvents(); len(got) != 1 {
		t.Errorf("event without a request id ingested twice produced %d rows, want 1", len(got))
	}
}
