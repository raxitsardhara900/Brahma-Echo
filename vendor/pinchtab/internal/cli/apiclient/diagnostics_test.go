package apiclient

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

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

// The not-running guidance may only name mechanisms that exist: PINCHTAB_PORT
// is written into launched instances and read by nothing, so following it left
// the server on the config port.
func TestCheckServerAndGuideNamesTheWorkingPortOverride(t *testing.T) {
	var running bool
	output := captureStderr(t, func() {
		running = CheckServerAndGuide(&http.Client{}, "http://127.0.0.1:1", "")
	})

	if running {
		t.Fatal("CheckServerAndGuide() = true against a closed port")
	}
	if strings.Contains(output, "PINCHTAB_PORT") {
		t.Errorf("guidance still advertises PINCHTAB_PORT, which nothing reads:\n%s", output)
	}
	if !strings.Contains(output, "pinchtab server --port") {
		t.Errorf("guidance does not name the --port flag:\n%s", output)
	}
}
