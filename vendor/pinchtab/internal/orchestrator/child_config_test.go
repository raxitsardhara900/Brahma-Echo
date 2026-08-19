package orchestrator

import (
	"os"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

// Every spawned bridge reads the file this writes, so the operator's log level has
// to survive the write. This is the orchestrator half of the chain — the child
// bridge's half, that it resolves the level it reads, lives in cmd/pinchtab.
func TestWriteChildConfigCarriesTheLogLevel(t *testing.T) {
	stateDir := t.TempDir()
	o := fakeOrch(&config.RuntimeConfig{
		Port:           "9867",
		DefaultBrowser: config.BrowserChrome,
		LogLevel:       "warn",
	})

	path, err := o.writeChildConfig(nil, "9999", 12345, "/tmp/profile", stateDir, true, nil, nil)
	if err != nil {
		t.Fatalf("write child config: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"logLevel": "warn"`) {
		t.Fatalf("child config on disk does not carry the level, so the spawned bridge cannot read it:\n%s", raw)
	}
}

// An unset level must not write a value, or a child inherits a level the operator
// never chose and the bridge can no longer tell "unset" from "configured info".
func TestWriteChildConfigOmitsAnUnsetLogLevel(t *testing.T) {
	stateDir := t.TempDir()
	o := fakeOrch(&config.RuntimeConfig{
		Port:           "9867",
		DefaultBrowser: config.BrowserChrome,
	})

	path, err := o.writeChildConfig(nil, "9999", 12345, "/tmp/profile", stateDir, true, nil, nil)
	if err != nil {
		t.Fatalf("write child config: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "logLevel") {
		t.Fatalf("child config carries a logLevel key with none configured:\n%s", raw)
	}
}
