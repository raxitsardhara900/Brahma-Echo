package bridge

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCrashSnapshotRecentIsAnArrayWhenEmptyAndWhenPopulated(t *testing.T) {
	ResetCrashMonitoringForTests()
	t.Cleanup(ResetCrashMonitoringForTests)

	empty, err := json.Marshal(CrashSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"recent":[]`) {
		t.Errorf("empty snapshot = %s, want recent to marshal as [] like failures.recent and the documented example, not null", empty)
	}

	RecordCrashForTests(CrashEvent{Time: time.Unix(1700000000, 0).UTC(), TabID: "tab1", Reason: "targetCrashed"})

	var populated struct {
		Total  uint64            `json:"total"`
		Recent []json.RawMessage `json:"recent"`
	}
	raw, err := json.Marshal(CrashSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &populated); err != nil {
		t.Fatalf("populated snapshot %s did not decode with recent as an array: %v", raw, err)
	}
	if populated.Total != 1 {
		t.Errorf("total = %d, want 1", populated.Total)
	}
	if len(populated.Recent) != 1 {
		t.Fatalf("recent = %s, want the one recorded event", raw)
	}
	for _, field := range []string{`"tabId":"tab1"`, `"reason":"targetCrashed"`} {
		if !strings.Contains(string(populated.Recent[0]), field) {
			t.Errorf("event = %s, want it to carry %s", populated.Recent[0], field)
		}
	}
}
