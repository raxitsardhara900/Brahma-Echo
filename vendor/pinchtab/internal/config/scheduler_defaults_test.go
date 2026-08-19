package config

import (
	"path/filepath"
	"strconv"
	"testing"
)

// Every scheduler knob answers, and it answers the value this package declares —
// derived from DefaultSchedulerConfig rather than retyped, so a changed default
// cannot leave a stale expectation passing here.
func TestSchedulerKeysAnswerTheDeclaredDefaults(t *testing.T) {
	writeConfigForGet(t, `{}`)

	defaults := DefaultSchedulerConfig()
	for path, want := range map[string]string{
		"scheduler.enabled":             strconv.FormatBool(defaults.Enabled),
		"scheduler.strategy":            defaults.Strategy,
		"scheduler.maxQueueSize":        strconv.Itoa(defaults.MaxQueueSize),
		"scheduler.maxPerAgent":         strconv.Itoa(defaults.MaxPerAgent),
		"scheduler.maxInflight":         strconv.Itoa(defaults.MaxInflight),
		"scheduler.maxPerAgentInflight": strconv.Itoa(defaults.MaxPerAgentFlight),
		"scheduler.resultTTLSec":        strconv.Itoa(defaults.ResultTTLSec),
		"scheduler.workerCount":         strconv.Itoa(defaults.WorkerCount),
		"scheduler.maxBatchSize":        strconv.Itoa(defaults.MaxBatchSize),
	} {
		if got := effectiveValue(t, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestSchedulerKeysKeepWhatTheFileSets(t *testing.T) {
	writeConfigForGet(t, `{"scheduler":{"maxInflight":7,"strategy":"strict-fifo","workerCount":9,"maxBatchSize":5}}`)

	for path, want := range map[string]string{
		"scheduler.maxInflight":  "7",
		"scheduler.strategy":     "strict-fifo",
		"scheduler.workerCount":  "9",
		"scheduler.maxBatchSize": "5",
	} {
		if got := effectiveValue(t, path); got != want {
			t.Errorf("%s = %q, want the configured %q", path, got, want)
		}
	}
}

// A configured zero has always meant "use the default" for these knobs — never
// unlimited. The reader must say so too, or it reports a queue size nothing enforces.
func TestSchedulerZeroMeansTheDefaultEverywhere(t *testing.T) {
	writeConfigForGet(t, `{"scheduler":{"maxQueueSize":0,"maxPerAgent":0,"maxInflight":0,"maxPerAgentInflight":0,"resultTTLSec":0,"workerCount":0,"maxBatchSize":0}}`)

	defaults := DefaultSchedulerConfig()
	cfg, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Scheduler != defaults {
		t.Errorf("runtime scheduler config = %+v, want the defaults %+v", cfg.Scheduler, defaults)
	}
	if got := effectiveValue(t, "scheduler.maxInflight"); got != strconv.Itoa(defaults.MaxInflight) {
		t.Errorf("scheduler.maxInflight = %q with an explicit 0 in the file, want the default %d", got, defaults.MaxInflight)
	}
}

// A negative value is the same case as zero, and it is the one a validator would let
// through as "set".
func TestSchedulerNegativeValuesFallBackToTheDefault(t *testing.T) {
	writeConfigForGet(t, `{"scheduler":{"workerCount":-4}}`)

	defaults := DefaultSchedulerConfig()
	if got := effectiveValue(t, "scheduler.workerCount"); got != strconv.Itoa(defaults.WorkerCount) {
		t.Errorf("scheduler.workerCount = %q with -4 in the file, want the default %d", got, defaults.WorkerCount)
	}
}

// The tab policy defaults already lived in this package; they were simply invisible to
// the reader because the writer omits a vanilla block on purpose.
func TestTabPolicyKeysAnswerTheRuntimeDefaults(t *testing.T) {
	writeConfigForGet(t, `{}`)

	cfg, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	for path, want := range map[string]string{
		"instanceDefaults.tabPolicy.eviction":      cfg.TabEvictionPolicy,
		"instanceDefaults.tabPolicy.lifecycle":     cfg.TabLifecyclePolicy,
		"instanceDefaults.tabPolicy.closeDelaySec": strconv.Itoa(int(cfg.TabCloseDelay.Seconds())),
	} {
		if want == "" || want == "0" {
			t.Fatalf("%s has no runtime default to compare against (%q), so this test proves nothing", path, want)
		}
		if got := effectiveValue(t, path); got != want {
			t.Errorf("%s = %q, want the runtime's %q", path, got, want)
		}
	}
}

func TestTabPolicyKeysKeepWhatTheFileSets(t *testing.T) {
	writeConfigForGet(t, `{"instanceDefaults":{"tabPolicy":{"eviction":"reject","lifecycle":"close_idle","closeDelaySec":42}}}`)

	for path, want := range map[string]string{
		"instanceDefaults.tabPolicy.eviction":      "reject",
		"instanceDefaults.tabPolicy.lifecycle":     "close_idle",
		"instanceDefaults.tabPolicy.closeDelaySec": "42",
	} {
		if got := effectiveValue(t, path); got != want {
			t.Errorf("%s = %q, want the configured %q", path, got, want)
		}
	}
}

// The activity log directory derives from the configured state dir, through the same
// single owner the store is handed.
func TestActivityStateDirAnswersTheDerivedLogDir(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "mystate")
	writeConfigForGet(t, `{"server":{"stateDir":`+quoteJSON(stateDir)+`}}`)

	want := filepath.Join(stateDir, "activity")
	if got := effectiveValue(t, "observability.activity.stateDir"); got != want {
		t.Errorf("observability.activity.stateDir = %q, want %q", got, want)
	}
}

// applyFileConfig never copies this key into the runtime — activity logs always live
// under server.stateDir so two instances cannot share one log dir. A reader that
// echoed the file would name a directory nothing writes to.
func TestActivityStateDirIgnoresAConfiguredOverride(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "mystate")
	writeConfigForGet(t, `{"server":{"stateDir":`+quoteJSON(stateDir)+`},
		"observability":{"activity":{"stateDir":`+quoteJSON(filepath.Join(root, "elsewhere"))+`}}}`)

	want := filepath.Join(stateDir, "activity")
	got := effectiveValue(t, "observability.activity.stateDir")
	if got != want {
		t.Errorf("observability.activity.stateDir = %q, want the directory in effect %q", got, want)
	}
	if got == filepath.Join(root, "elsewhere") {
		t.Error("the reader echoed the configured override, which the runtime discards")
	}
}
