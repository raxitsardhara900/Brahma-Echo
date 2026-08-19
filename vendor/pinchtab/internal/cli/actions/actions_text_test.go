package actions

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTextCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("raw", false, "")
	cmd.Flags().Bool("full", false, "")
	cmd.Flags().String("tab", "", "")
	cmd.Flags().String("frame", "", "")
	cmd.Flags().String("selector", "", "")
	cmd.Flags().Bool("json", false, "")
	return cmd
}

func TestText(t *testing.T) {
	m := newMockServer()
	m.response = `{"url":"https://pinchtab.com","title":"Example","text":"Hello"}`
	defer m.close()
	client := m.server.Client()

	cmd := newTextCmd()
	Text(client, m.base(), "", cmd, nil)
	if m.lastPath != "/text" {
		t.Errorf("expected /text, got %s", m.lastPath)
	}
}

func TestTextRaw(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newTextCmd()
	_ = cmd.Flags().Set("raw", "true")
	Text(client, m.base(), "", cmd, nil)
	if !strings.Contains(m.lastQuery, "mode=raw") {
		t.Errorf("expected mode=raw, got %s", m.lastQuery)
	}
	if !strings.Contains(m.lastQuery, "format=text") {
		t.Errorf("expected format=text, got %s", m.lastQuery)
	}
}

func TestTextFull(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newTextCmd()
	_ = cmd.Flags().Set("full", "true")
	Text(client, m.base(), "", cmd, nil)
	if !strings.Contains(m.lastQuery, "mode=raw") {
		t.Errorf("expected --full to set mode=raw, got %s", m.lastQuery)
	}
	if !strings.Contains(m.lastQuery, "format=text") {
		t.Errorf("expected --full to set format=text, got %s", m.lastQuery)
	}
}

func TestTextFrame(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newTextCmd()
	_ = cmd.Flags().Set("frame", "FRAME123")
	Text(client, m.base(), "", cmd, nil)
	if !strings.Contains(m.lastQuery, "frameId=FRAME123") {
		t.Errorf("expected frameId=FRAME123, got %s", m.lastQuery)
	}
}

func TestTextTab(t *testing.T) {
	m := newMockServer()
	defer m.close()
	client := m.server.Client()

	cmd := newTextCmd()
	_ = cmd.Flags().Set("tab", "TAB1")
	Text(client, m.base(), "", cmd, nil)
	if !strings.Contains(m.lastQuery, "tabId=TAB1") {
		t.Errorf("expected tabId=TAB1, got %s", m.lastQuery)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = writer

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&buf, reader)
		_ = reader.Close()
		done <- copyErr
	}()

	defer func() { os.Stderr = oldStderr }()

	fn()

	_ = writer.Close()
	if err := <-done; err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

func runTextAgainst(t *testing.T, response string) (stdout, stderr string) {
	t.Helper()
	m := newMockServer()
	m.response = response
	defer m.close()

	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			Text(m.server.Client(), m.base(), "", newTextCmd(), nil)
		})
	})
	return stdout, stderr
}

func TestTextFallbackNotesOnStderrAndKeepsStdoutClean(t *testing.T) {
	page := "PinchTab — browser control for AI agents\nInstall: curl -fsSL https://pinchtab.com/install.sh | sh"
	body, err := json.Marshal(map[string]any{
		"text":       page,
		"extraction": "readability_fallback",
		"textLength": len(page),
		"rawLength":  len(page),
	})
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr := runTextAgainst(t, string(body))

	if stdout != page+"\n" {
		t.Errorf("stdout = %q, want the page text alone", stdout)
	}
	if !strings.Contains(stderr, "--full") {
		t.Errorf("stderr = %q, want a note pointing at --full", stderr)
	}
}

func TestTextNoNoteWhenReadabilityHeld(t *testing.T) {
	body := `{"text":"Article body","extraction":"readability","textLength":12,"rawLength":13}`

	stdout, stderr := runTextAgainst(t, body)

	if stdout != "Article body\n" {
		t.Errorf("stdout = %q, want the article text", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want no note when readability held", stderr)
	}
}
