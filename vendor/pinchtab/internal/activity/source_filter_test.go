package activity

import (
	"testing"
	"time"
)

// The source a client sends in X-PinchTab-Source is stored verbatim, but the
// per-source log file is named with the normalized form. queryFiles compares
// normalized names while Filter.matches compared raw ones, so a query that
// named the source the way it appears on disk read the right file and then
// discarded every event in it.
func TestQuerySourceFilterIsNormalized(t *testing.T) {
	recorded := []string{"MCP", "mcp", " Mcp "}
	for _, source := range recorded {
		t.Run(source, func(t *testing.T) {
			s, err := NewStore(t.TempDir(), 7)
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}
			if err := s.Record(Event{
				Source:    source,
				Timestamp: time.Now().UTC(),
				Method:    "GET",
				Path:      "/snapshot",
				Status:    200,
			}); err != nil {
				t.Fatalf("Record: %v", err)
			}

			got, err := s.Query(Filter{Source: "mcp"})
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("Query(source=mcp) returned %d events for a source recorded as %q, want 1", len(got), source)
			}
		})
	}
}

// A source whose normalized name merely shares a prefix with the queried one
// must not be returned.
func TestQuerySourceFilterDoesNotMatchPrefix(t *testing.T) {
	s, err := NewStore(t.TempDir(), 7)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	now := time.Now().UTC()
	for _, source := range []string{"mcp", "mcp-extra"} {
		if err := s.Record(Event{Source: source, Timestamp: now, Method: "GET", Path: "/x", Status: 200}); err != nil {
			t.Fatalf("Record(%s): %v", source, err)
		}
	}

	got, err := s.Query(Filter{Source: "mcp"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Query(source=mcp) returned %d events, want 1 (mcp-extra must not match)", len(got))
	}
	if got[0].Source != "mcp" {
		t.Errorf("got source %q, want mcp", got[0].Source)
	}
}
