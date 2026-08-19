package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

func modeConfigPhases() map[string]func() {
	return map[string]func(){
		"server": maybeRunWizard,
		"bridge": func() {},
	}
}

func writeOperatorConfig(t *testing.T) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(configPath, []byte(`{"server":{"port":"18274"},"instanceDefaults":{"mode":"headless"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func TestBothModesRefuseToProvisionATokenIntoAnOperatorConfig(t *testing.T) {
	for mode, configPhase := range modeConfigPhases() {
		t.Run(mode, func(t *testing.T) {
			configPath := writeOperatorConfig(t)
			t.Setenv("PINCHTAB_CONFIG", configPath)
			t.Setenv("PINCHTAB_TOKEN", "")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}

			configPhase()
			refusal := ensureMandatoryToken()

			if !errors.Is(refusal, config.ErrOperatorConfigToken) {
				t.Fatalf("%s mode: err = %v, want the operator-config refusal; a mode whose config phase provisions first makes the guard unreachable", mode, refusal)
			}
			if !strings.Contains(refusal.Error(), "add server.token or set PINCHTAB_TOKEN") {
				t.Errorf("%s mode: refusal %q lost its remedies", mode, refusal)
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("%s mode: the operator's config changed on a refused start:\nbefore: %s\nafter:  %s", mode, before, after)
			}
		})
	}
}

func TestAnEnvTokenStartsBothModesWithoutTouchingTheOperatorConfig(t *testing.T) {
	for mode, configPhase := range modeConfigPhases() {
		t.Run(mode, func(t *testing.T) {
			configPath := writeOperatorConfig(t)
			t.Setenv("PINCHTAB_CONFIG", configPath)
			t.Setenv("PINCHTAB_TOKEN", "env-token")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}

			configPhase()
			if err := ensureMandatoryToken(); err != nil {
				t.Fatalf("%s mode: PINCHTAB_TOKEN must satisfy the token requirement, got %v", mode, err)
			}

			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("%s mode: the operator's config changed under an env token:\nbefore: %s\nafter:  %s", mode, before, after)
			}
		})
	}
}

func TestTheDefaultPathStillSelfProvisionsAndSaysSo(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("PINCHTAB_CONFIG", "")
	t.Setenv("PINCHTAB_TOKEN", "")

	stderr := captureStderr(t, func() {
		if err := ensureMandatoryToken(); err != nil {
			t.Fatalf("a first run on the default path must self-provision, got %v", err)
		}
	})

	fc, configPath, err := config.LoadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if fc == nil || strings.TrimSpace(fc.Server.Token) == "" {
		t.Fatalf("no token persisted at %s", configPath)
	}
	if !strings.Contains(stderr, "server.token") {
		t.Errorf("stderr %q does not name the generated server.token; a silent credential write is what this card removed", stderr)
	}
}

func TestRecordConfigVersionNamesAGeneratedToken(t *testing.T) {
	for _, tc := range []struct {
		name           string
		announce       bool
		tokenGenerated bool
		wantSays       []string
		wantSilent     bool
	}{
		{name: "version and token", announce: true, tokenGenerated: true, wantSays: []string{"configVersion", "server.token"}},
		{name: "version only", announce: true, tokenGenerated: false, wantSays: []string{"configVersion"}},
		{name: "token without version announcement", announce: false, tokenGenerated: true, wantSays: []string{"server.token"}},
		{name: "quiet when nothing was generated", announce: false, tokenGenerated: false, wantSilent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "cfg.json")
			cfg := &config.FileConfig{}
			stderr := captureStderr(t, func() {
				if !recordConfigVersion(cfg, configPath, tc.announce, tc.tokenGenerated) {
					t.Fatalf("recordConfigVersion failed")
				}
			})
			if tc.wantSilent && stderr != "" {
				t.Fatalf("stderr = %q, want silence", stderr)
			}
			for _, says := range tc.wantSays {
				if !strings.Contains(stderr, says) {
					t.Errorf("stderr = %q, want it to name %q; the line telling the operator their file was touched must not omit the secret half of the write", stderr, says)
				}
			}
		})
	}
}
