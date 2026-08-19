package actions

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("interactive", false, "")
	cmd.Flags().Bool("compact", false, "")
	cmd.Flags().Bool("text", false, "")
	cmd.Flags().Bool("diff", false, "")
	cmd.Flags().String("selector", "", "")
	cmd.Flags().String("max-tokens", "", "")
	cmd.Flags().String("depth", "", "")
	cmd.Flags().String("tab", "", "")
	return cmd
}

func TestSnapshot(t *testing.T) {
	m := newMockServer()
	m.response = `[{"ref":"e0","role":"button","name":"Submit"}]`
	defer m.close()
	client := m.server.Client()

	cmd := newSnapshotCmd()
	_ = cmd.Flags().Set("interactive", "true")
	_ = cmd.Flags().Set("compact", "true")
	Snapshot(client, m.base(), "", cmd, "")
	if m.lastMethod != "GET" {
		t.Errorf("expected GET, got %s", m.lastMethod)
	}
	if m.lastPath != "/snapshot" {
		t.Errorf("expected /snapshot, got %s", m.lastPath)
	}
	if !strings.Contains(m.lastQuery, "filter=interactive") {
		t.Errorf("expected filter=interactive in query, got %s", m.lastQuery)
	}
	if !strings.Contains(m.lastQuery, "format=compact") {
		t.Errorf("expected format=compact in query, got %s", m.lastQuery)
	}
}

func TestSnapshotDiff(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newSnapshotCmd()
	_ = cmd.Flags().Set("diff", "true")
	_ = cmd.Flags().Set("selector", "main")
	_ = cmd.Flags().Set("max-tokens", "2000")
	_ = cmd.Flags().Set("depth", "5")
	Snapshot(client, m.base(), "", cmd, "")
	if !strings.Contains(m.lastQuery, "diff=true") {
		t.Errorf("expected diff=true, got %s", m.lastQuery)
	}
	if !strings.Contains(m.lastQuery, "selector=main") {
		t.Errorf("expected selector=main, got %s", m.lastQuery)
	}
	if !strings.Contains(m.lastQuery, "maxTokens=2000") {
		t.Errorf("expected maxTokens=2000, got %s", m.lastQuery)
	}
	if !strings.Contains(m.lastQuery, "depth=5") {
		t.Errorf("expected depth=5, got %s", m.lastQuery)
	}
}

func TestSnapshotTabId(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newSnapshotCmd()
	_ = cmd.Flags().Set("tab", "ABC123")
	Snapshot(client, m.base(), "", cmd, "")
	if !strings.Contains(m.lastQuery, "tabId=ABC123") {
		t.Errorf("expected tabId=ABC123, got %s", m.lastQuery)
	}
}

func TestSnapshotSelectorOverride(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newSnapshotCmd()
	Snapshot(client, m.base(), "", cmd, "#main")
	if !strings.Contains(m.lastQuery, "selector=%23main") {
		t.Errorf("expected selector override, got %s", m.lastQuery)
	}
}
