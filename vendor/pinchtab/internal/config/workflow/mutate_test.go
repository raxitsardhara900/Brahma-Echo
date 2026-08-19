package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func TestInitDefaultConfigIncludesSchema(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "pinchtab", "config.json")

	if err := InitDefaultConfig(configPath); err != nil {
		t.Fatalf("InitDefaultConfig() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("config JSON is invalid: %v", err)
	}
	if raw["$schema"] != config.CurrentConfigSchemaURL() {
		t.Fatalf("$schema = %q, want %q", raw["$schema"], config.CurrentConfigSchemaURL())
	}
}
