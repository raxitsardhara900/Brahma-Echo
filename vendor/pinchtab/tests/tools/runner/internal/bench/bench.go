// Command runner is the Go port of
// tests/benchmark/scripts/run-api-benchmark.ts.
//
// Exit codes:
//
//	0  success (or --help)
//	1  argument/setup error
//	2  max turns reached
//	3  idle turn limit reached
//	4  budget exceeded
package bench

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) > 0 {
		switch argv[0] {
		case "verify-step":
			return RunVerifyStep(argv[1:], stdout, stderr)
		case "record-step":
			return RunRecordStep(argv[1:], stdout, stderr)
		case "step-end":
			return RunStepEnd(argv[1:], stdout, stderr)
		}
	}

	args, err := ParseArgs(argv)
	if err != nil {
		if errors.Is(err, errHelp) {
			WriteUsage(stdout)
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "runner: %v\n\n", err)
		WriteUsage(stderr)
		return 1
	}

	if args.DryRun {
		_, _ = fmt.Fprint(stdout, formatPlan(args))
		return 0
	}

	toolsDir := resolveToolsDir()
	benchmarkDir := resolveBenchmarkDir()
	provider := resolveProvider(args)
	model := resolveModel(provider, args.Model)
	groups := resolveGroupsFromArgs(args)

	// Validate API keys
	if err := validateAPIKeys(provider); err != nil {
		_, _ = fmt.Fprintf(stderr, "runner: %v\n", err)
		return 1
	}

	// Setup container (unless --skip-init)
	if !args.SkipInit {
		if err := setupLaneContainer(args.Lane, toolsDir, stdout, stderr); err != nil {
			_, _ = fmt.Fprintf(stderr, "runner: %v\n", err)
			return 1
		}
	}

	// Initialize report (results go in benchmarkDir)
	resultsDir := filepath.Join(benchmarkDir, "results")
	reportFile := args.ReportFile
	if reportFile == "" {
		if args.SkipInit {
			reportFile = resolveReportPath(resultsDir, args.Lane)
		} else {
			var err error
			reportFile, err = initializeLaneGo(resultsDir, args.Lane, provider, model, stdout, stderr)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "runner: %v\n", err)
				return 1
			}
		}
	}

	_, _ = fmt.Fprintf(stdout, "[benchmark-runner] provider=%s model=%s lane=%s groups=%s report=%s\n",
		provider, model, args.Lane, formatGroups(groups), reportFile)

	// Harness-generated start-of-run summary
	PrintStartBanner(stdout, args.Lane, benchmarkDir, groups, model, reportFile)

	runner := createRunner(provider, model, args)
	shell, err := NewPersistentShell(toolsDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "runner: shell init failed: %v\n", err)
		return 1
	}
	defer shell.Close(false)

	cfg := LoopConfig{
		Lane:            args.Lane,
		Provider:        provider,
		Model:           model,
		Groups:          groups,
		ReportFile:      reportFile,
		MaxTurns:        args.MaxTurns,
		MaxIdleTurns:    args.MaxIdleTurns,
		TimeoutSeconds:  args.TimeoutSeconds,
		TurnDelayMs:     args.TurnDelayMs,
		Finalize:        args.Finalize,
		ToolsDir:        toolsDir,
		BenchmarkDir:    benchmarkDir,
		CommandLogFile:  filepath.Join(resultsDir, commandLogName(args.Lane)),
		MaxInputTokens:  args.MaxInputTokens,
		MaxOutputTokens: args.MaxOutputTokens,
		TerseSummary:    args.TerseSummary,
		Verbose:         args.Verbose,
		Stdout:          stdout,
		Stderr:          stderr,
	}

	result := RunLoop(cfg, runner, shell)
	return result.ExitCode
}

func resolveRepoRoot() string {
	cwd, _ := os.Getwd()
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd
		}
		dir = parent
	}
}

func resolveToolsDir() string {
	return filepath.Join(resolveRepoRoot(), "tests", "tools")
}

func resolveBenchmarkDir() string {
	return filepath.Join(resolveRepoRoot(), "tests", "benchmark")
}

func resolveProvider(args Args) string {
	if args.Provider != ProviderUnset {
		return string(args.Provider)
	}
	hasOpenAI := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
	hasAnthropic := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
	if hasOpenAI && hasAnthropic {
		_, _ = fmt.Fprintln(os.Stderr, "warning: multiple providers configured, defaulting to anthropic")
		return "anthropic"
	}
	if hasOpenAI {
		return "openai"
	}
	if hasAnthropic {
		return "anthropic"
	}
	return "anthropic"
}

func resolveModel(provider, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if provider == "openai" {
		if m := os.Getenv("OPENAI_MODEL"); m != "" {
			return m
		}
		return "gpt-5"
	}
	if m := os.Getenv("ANTHROPIC_MODEL"); m != "" {
		return m
	}
	return "claude-haiku-4-5-20251001"
}

func resolveGroupsFromArgs(args Args) []int {
	if len(args.Groups) > 0 {
		return args.Groups
	}
	switch args.Profile {
	case "common10":
		return []int{0, 1, 2, 3}
	default:
		return nil
	}
}

func commandLogName(lane Lane) string {
	if lane == LaneAgentBrowser {
		return "agent_browser_commands.ndjson"
	}
	return "pinchtab_commands.ndjson"
}

func resolveReportPath(benchDir string, lane Lane) string {
	ptrFile := filepath.Join(benchDir, "results", "current_pinchtab_report.txt")
	if lane == LaneAgentBrowser {
		ptrFile = filepath.Join(benchDir, "results", "current_agent_browser_report.txt")
	}
	data, err := os.ReadFile(ptrFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func runnerSource(provider string) string {
	if provider == "openai" {
		return "openai-responses"
	}
	return "anthropic-messages"
}

func createRunner(provider, model string, args Args) Runner {
	promptCaching := !args.NoPromptCaching
	if provider == "openai" {
		apiKey := os.Getenv("OPENAI_API_KEY")
		return NewOpenAIRunner(apiKey, model, args.MaxTokens, args.Temperature, promptCaching)
	}
	if provider == "fake" {
		return NewFakeRunner(model, nil)
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	return NewAnthropicRunner(apiKey, model, args.MaxTokens, args.Temperature, promptCaching)
}

func formatPlan(a Args) string {
	var b strings.Builder
	b.WriteString("runner (Go) — resolved plan\n")
	_, _ = fmt.Fprintf(&b, "  lane:              %s\n", a.Lane)
	_, _ = fmt.Fprintf(&b, "  provider:          %s\n", stringOr(string(a.Provider), "(auto-detect from env)"))
	_, _ = fmt.Fprintf(&b, "  model:             %s\n", stringOr(a.Model, "(provider default)"))
	_, _ = fmt.Fprintf(&b, "  groups:            %s\n", formatGroups(a.Groups))
	_, _ = fmt.Fprintf(&b, "  profile:           %s\n", stringOr(a.Profile, "(none)"))
	_, _ = fmt.Fprintf(&b, "  max-tokens:        %d\n", a.MaxTokens)
	_, _ = fmt.Fprintf(&b, "  temperature:       %g\n", a.Temperature)
	_, _ = fmt.Fprintf(&b, "  max-turns:         %d\n", a.MaxTurns)
	_, _ = fmt.Fprintf(&b, "  max-idle-turns:    %d\n", a.MaxIdleTurns)
	_, _ = fmt.Fprintf(&b, "  timeout-seconds:   %d\n", a.TimeoutSeconds)
	_, _ = fmt.Fprintf(&b, "  turn-delay-ms:     %d\n", a.TurnDelayMs)
	_, _ = fmt.Fprintf(&b, "  report-file:       %s\n", stringOr(a.ReportFile, "(auto-generated)"))
	_, _ = fmt.Fprintf(&b, "  skip-init:         %t\n", a.SkipInit)
	_, _ = fmt.Fprintf(&b, "  no-prompt-caching: %t\n", a.NoPromptCaching)
	_, _ = fmt.Fprintf(&b, "  finalize:          %t\n", a.Finalize)
	_, _ = fmt.Fprintf(&b, "  dry-run:           %t\n", a.DryRun)
	_, _ = fmt.Fprintf(&b, "  index-file:        %s\n", stringOr(a.IndexFile, "(default)"))
	return b.String()
}

func stringOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func formatGroups(groups []int) string {
	if len(groups) == 0 {
		return "all"
	}
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = fmt.Sprintf("%d", g)
	}
	return strings.Join(parts, ",")
}
