package report

import (
	"testing"
)

func TestSecurityLevelColor(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{level: "LOCKED", want: string(colorSuccess)},
		{level: "GUARDED", want: string(colorWarning)},
		{level: "ELEVATED", want: string(colorWarning)},
		{level: "EXPOSED", want: string(colorDanger)},
		{level: "UNKNOWN", want: string(colorDanger)},
	}

	for _, tt := range tests {
		if got := SecurityLevelColor(tt.level); got != tt.want {
			t.Fatalf("SecurityLevelColor(%q) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestDefaultListenStatus(t *testing.T) {
	tests := []struct {
		mode     string
		explicit string
		want     string
	}{
		{mode: "menu", want: "stopped"},
		{mode: "server", want: "starting"},
		{mode: "bridge", want: "starting"},
		{mode: "mcp", want: "starting"},
		{mode: "server", explicit: "running", want: "running"},
		{mode: "other", want: ""},
	}

	for _, tt := range tests {
		if got := defaultListenStatus(tt.mode, tt.explicit); got != tt.want {
			t.Fatalf("defaultListenStatus(%q, %q) = %q, want %q", tt.mode, tt.explicit, got, tt.want)
		}
	}
}

func TestFormatListenValuePlain(t *testing.T) {
	got := formatListenValue("127.0.0.1:9867", "")
	if got != "127.0.0.1:9867" {
		t.Fatalf("formatListenValue() = %q, want plain address", got)
	}
}
