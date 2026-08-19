package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/pinchtab/pinchtab/internal/browsers/all"
	"github.com/pinchtab/pinchtab/internal/browsers/runtimekit"
	"github.com/pinchtab/pinchtab/internal/config"
	"github.com/pinchtab/pinchtab/internal/safelog"
)

func TestValidateBridgeCDPURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "browser websocket", raw: "ws://127.0.0.1:9222/devtools/browser/abc"},
		{name: "http origin", raw: "http://127.0.0.1:9222"},
		{name: "json version", raw: "https://cdp.example/json/version"},
		{name: "page websocket rejected", raw: "ws://127.0.0.1:9222/devtools/page/abc", wantErr: true},
		{name: "websocket without browser path rejected", raw: "ws://127.0.0.1:9222", wantErr: true},
		{name: "missing scheme rejected", raw: "127.0.0.1:9222", wantErr: true},
		{name: "unsupported scheme rejected", raw: "ftp://127.0.0.1:9222", wantErr: true},
		{name: "bad http path rejected", raw: "http://127.0.0.1:9222/devtools/page/abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateBridgeCDPURL(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveBridgeBrowser(t *testing.T) {
	tests := []struct {
		name        string
		browserFlag string
		configured  []string
		want        string
		wantErr     bool
	}{
		{name: "browser flag sets cloak", browserFlag: "cloak", want: "cloak"},
		{name: "browser flag sets chrome", browserFlag: "chrome", want: "chrome"},
		{name: "no flag returns empty", want: ""},
		{name: "invalid browser returns error", browserFlag: "netscape", wantErr: true},
		{name: "configured browser accepted", browserFlag: "my-custom", configured: []string{"my-custom"}, want: "my-custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBridgeBrowser(tt.browserFlag, tt.configured)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBridgeAttachChildFlagContract(t *testing.T) {
	for _, name := range []string{"cdp-attach", "browser", "remote-browser-name"} {
		if bridgeCmd.Flags().Lookup(name) == nil {
			t.Errorf("bridge command missing child flag %q", name)
		}
	}
	if bridgeCmd.Flags().Lookup("browser-provider") != nil {
		t.Error("bridge command must not register obsolete browser-provider flag")
	}
}

func bridgeTargetsConfig() *config.RuntimeConfig {
	return &config.RuntimeConfig{
		DefaultBrowser: config.BrowserCloak,
		DefaultTarget:  "cloak-primary",
		Targets: config.BrowserTargetsConfig{
			"chrome-alt":    {Provider: config.BrowserChrome, Binary: "/tmp/pinchtab-test-chrome"},
			"cloak-primary": {Provider: config.BrowserCloak, Binary: "/tmp/pinchtab-test-cloak"},
		},
	}
}

func TestApplyBridgeBrowserTargetOverridesDefaultTarget(t *testing.T) {
	cfg := bridgeTargetsConfig()

	applyBridgeBrowserTarget(cfg, config.BrowserChrome)

	if cfg.DefaultTarget != "chrome-alt" {
		t.Fatalf("DefaultTarget = %q, want chrome-alt", cfg.DefaultTarget)
	}
	effective := runtimekit.ResolveEffectiveBrowser(cfg)
	if effective.ID != config.BrowserChrome {
		t.Fatalf("resolved provider = %q, want chrome", effective.ID)
	}
	if effective.Binary != "/tmp/pinchtab-test-chrome" {
		t.Fatalf("resolved binary = %q, want the chrome target binary", effective.Binary)
	}
}

func TestApplyBridgeBrowserTargetClearsUnmatchedDefaultTarget(t *testing.T) {
	cfg := bridgeTargetsConfig()

	applyBridgeBrowserTarget(cfg, config.BrowserGhostChrome)

	if cfg.DefaultTarget != "" {
		t.Fatalf("DefaultTarget = %q, want cleared", cfg.DefaultTarget)
	}
	if got := runtimekit.ResolveEffectiveBrowser(cfg).ID; got != config.BrowserGhostChrome {
		t.Fatalf("resolved provider = %q, want ghost-chrome", got)
	}
}

// The bridge holds the CDP session, so it owns the logging a debug level is
// actually wanted for. It reads the same server.logLevel the server does, and the
// orchestrator writes that key into every child config — so before this was wired,
// the setting looked plumbed end to end and was dropped at the last step.
func TestBridgeResolvesTheConfiguredLogLevel(t *testing.T) {
	t.Cleanup(func() { safelog.SetLevel(safelog.DefaultLevel) })

	for _, tc := range []struct {
		name        string
		configLevel string
		flag        string
		want        slog.Level
	}{
		{name: "flagless bridge takes the configured level", configLevel: "warn", want: slog.LevelWarn},
		{name: "flag overrides the configured level", configLevel: "warn", flag: "debug", want: slog.LevelDebug},
		{name: "blank flag leaves the configured level", configLevel: "warn", flag: "  ", want: slog.LevelWarn},
		{name: "neither is the default", want: safelog.DefaultLevel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			safelog.SetLevel(slog.LevelInfo)
			cfg := &config.RuntimeConfig{LogLevel: tc.configLevel}
			resolveLogLevel(cfg, tc.flag, false)
			if got := safelog.CurrentLevel(); got != tc.want {
				t.Errorf("level = %v, want %v", got, tc.want)
			}
		})
	}
}

// Parity with the server is a structural property, not a coincidence of two tables:
// both commands hand their (flag, config) pair to the same function, and neither
// decides anything itself. Asserting the resolved levels match would pass even if the
// bridge grew its own copy, so the source is where this is pinned.
//
// The source half now lives in TestTheCommandPackageSettlesTheLogLevelInOnePlace,
// which censuses the whole package by glob: one resolveLogLevel declaration, both
// commands calling it, and neither banned pattern outside the declaring file. Checking
// cmd_bridge.go by name here as well left every other command file unguarded for
// safelog.SetLevel — a stray call in cmd_server_ensure.go compiled green. What stays
// below is what only the bridge can assert: its own flag surface.
func TestBridgeAndServerResolveTheLogLevelThroughOneFunction(t *testing.T) {
	if bridgeCmd.Flags().Lookup("log-level") == nil {
		t.Error("bridge has no --log-level flag, so it cannot override the configured level the way --bind and --port do")
	}
	if bridgeCmd.Flags().Lookup("verbose") != nil {
		t.Error("bridge grew a -v; if that is wanted it must mean what it means on the server, and the flag help and docs must stop saying the bridge has none")
	}
	if help := bridgeCmd.Flags().Lookup("log-level").Usage; !strings.Contains(help, "server.logLevel") || !strings.Contains(help, "-v") {
		t.Errorf("--log-level help %q must name the config key it overrides and explain the missing -v", help)
	}
}

// The path that made the setting look plumbed: the orchestrator builds each child
// config through config.FileConfigFromRuntime and writes it under the instance
// state dir, and the child bridge then loads that file. This drives both halves —
// the written file and the load — so "carried but ignored" cannot come back.
func TestBridgeResolvesTheLevelFromAWrittenChildConfig(t *testing.T) {
	t.Cleanup(func() { safelog.SetLevel(safelog.DefaultLevel) })
	safelog.SetLevel(slog.LevelInfo)

	childConfig := config.FileConfigFromRuntime(&config.RuntimeConfig{
		Token:    "child-token",
		LogLevel: "warn",
	})
	encoded, err := json.MarshalIndent(childConfig, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"logLevel": "warn"`) {
		t.Fatalf("child config does not carry the level, so no bridge could read it: %s", encoded)
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINCHTAB_CONFIG", path)
	t.Setenv("PINCHTAB_TOKEN", "")

	cfg := loadConfig()
	resolveLogLevel(cfg, "", false)

	if got := safelog.CurrentLevel(); got != slog.LevelWarn {
		t.Fatalf("bridge started from a written child config resolved %v, want warn", got)
	}
}

// The window this card closed: the prologue validated flags between the load and
// the emit, so a bad --cdp-attach returned before the loader's warnings were ever
// written. Driving the real RunE is what makes that visible — the warning has to
// reach the log alongside the flag error, not instead of it.
func TestBridgeFlagErrorStillEmitsLoaderWarnings(t *testing.T) {
	writeRunConfig(t, `{"server":{"port":"9867"},"browsers":{"default":"not-a-browser"}}`)
	buf := captureRunLog(t)

	previous := bridgeCDPAttach
	bridgeCDPAttach = "garbage"
	t.Cleanup(func() { bridgeCDPAttach = previous })

	err := bridgeCmd.RunE(bridgeCmd, nil)
	if err == nil {
		t.Fatal("bridge --cdp-attach garbage returned no error, so this test is not exercising the early-return path")
	}
	if !strings.Contains(err.Error(), "--cdp-attach") {
		t.Fatalf("error = %v, want the --cdp-attach validation failure", err)
	}
	if out := buf.String(); !strings.Contains(out, "not a known browser") {
		t.Errorf("the flag error discarded the loader warning:\n%s", out)
	}
}
