package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.8.0", "0.8.0", 0},
		{"0.7.0", "0.8.0", -1},
		{"0.8.0", "0.7.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.8.1", "0.8.0", 1},
		{"0.8.0", "0.8.1", -1},
		{"1.0.0", "1.0.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNeedsWizard(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"empty version", "", true},
		{"old version", "0.7.0", true},
		{"current version", CurrentConfigVersion, false},
		{"future version", "1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &FileConfig{ConfigVersion: tt.version}
			if got := NeedsWizard(cfg); got != tt.want {
				t.Errorf("NeedsWizard(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsFirstRun(t *testing.T) {
	if !IsFirstRun(&FileConfig{}) {
		t.Error("expected IsFirstRun for empty config")
	}
	if IsFirstRun(&FileConfig{ConfigVersion: "0.8.0"}) {
		t.Error("expected not IsFirstRun for versioned config")
	}
}

func TestUserConfigDirLinuxAlwaysUsesLegacyPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific path test")
	}

	tmpHome, err := os.MkdirTemp("", "pinchtab-home-*")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpHome) }()

	t.Setenv("HOME", tmpHome)

	got := userConfigDir()
	want := filepath.Join(tmpHome, ".pinchtab")
	if got != want {
		t.Fatalf("userConfigDir() = %q, want Linux default path %q", got, want)
	}
}

func TestUserConfigDirDarwinAlwaysUsesLegacyPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific path test")
	}

	tmpHome, err := os.MkdirTemp("", "pinchtab-home-*")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpHome) }()

	t.Setenv("HOME", tmpHome)

	got := userConfigDir()
	want := filepath.Join(tmpHome, ".pinchtab")
	if got != want {
		t.Fatalf("userConfigDir() = %q, want macOS default path %q", got, want)
	}
}

func TestUserConfigDirWindowsUsesUserConfigDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific path test")
	}

	tmpHome, err := os.MkdirTemp("", "pinchtab-home-*")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpHome) }()

	configHome := filepath.Join(tmpHome, "AppData", "Roaming")
	t.Setenv("HOME", tmpHome)
	t.Setenv("AppData", configHome)
	t.Setenv("APPDATA", configHome)

	got := userConfigDir()
	want := filepath.Join(configHome, "pinchtab")
	if got != want {
		t.Fatalf("userConfigDir() = %q, want Windows default path %q", got, want)
	}
}

func TestProvisionFileTokenRefusesAnOperatorSuppliedConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(configPath, []byte(`{"server":{"port":"1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", configPath)

	fc := &FileConfig{}
	changed, err := ProvisionFileToken(fc, configPath)
	if !errors.Is(err, ErrOperatorConfigToken) {
		t.Fatalf("err = %v, want ErrOperatorConfigToken; a generated credential must not be written into a file the operator supplied", err)
	}
	if changed || fc.Server.Token != "" {
		t.Fatalf("changed = %v, token = %q; the refusal must not provision anyway", changed, fc.Server.Token)
	}
}

func TestProvisionFileTokenProvisionsWhenTheConfigIsNotOperatorSupplied(t *testing.T) {
	for _, tc := range []struct {
		name   string
		envVal func(dir string) string
	}{
		{"no PINCHTAB_CONFIG", func(string) string { return "" }},
		{"PINCHTAB_CONFIG names an absent file", func(dir string) string { return filepath.Join(dir, "cfg.json") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "cfg.json")
			t.Setenv("PINCHTAB_CONFIG", tc.envVal(dir))

			fc := &FileConfig{}
			changed, err := ProvisionFileToken(fc, configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !changed || strings.TrimSpace(fc.Server.Token) == "" {
				t.Fatalf("changed = %v, token = %q; a config the operator did not author still self-provisions", changed, fc.Server.Token)
			}
		})
	}
}

func TestProvisionFileTokenKeepsAnExistingToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", configPath)

	fc := &FileConfig{Server: ServerConfig{Token: "tok"}}
	changed, err := ProvisionFileToken(fc, configPath)
	if err != nil || changed {
		t.Fatalf("changed = %v, err = %v; a present token needs no decision at all", changed, err)
	}
}
