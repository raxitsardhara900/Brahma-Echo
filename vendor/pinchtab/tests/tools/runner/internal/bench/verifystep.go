package bench

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VerifyStepArgs struct {
	ReportType string
	ReportFile string
	Group      int
	Step       int
	Status     string
	Notes      string
}

func ParseVerifyStepArgs(argv []string) (VerifyStepArgs, error) {
	var args VerifyStepArgs
	i := 0
	for i < len(argv) && strings.HasPrefix(argv[i], "--") {
		switch argv[i] {
		case "--type":
			if i+1 >= len(argv) {
				return args, errors.New("--type requires a value")
			}
			args.ReportType = argv[i+1]
			i += 2
		case "--report-file":
			if i+1 >= len(argv) {
				return args, errors.New("--report-file requires a value")
			}
			args.ReportFile = argv[i+1]
			i += 2
		default:
			return args, fmt.Errorf("unknown option: %s", argv[i])
		}
	}

	positional := argv[i:]
	if len(positional) < 3 {
		return args, errors.New("usage: verify-step [--type TYPE] [--report-file PATH] <group> <step> <pass|fail|skip> [notes]")
	}

	var err error
	args.Group, err = parseInt(positional[0])
	if err != nil {
		return args, fmt.Errorf("invalid group: %w", err)
	}
	args.Step, err = parseInt(positional[1])
	if err != nil {
		return args, fmt.Errorf("invalid step: %w", err)
	}
	args.Status = positional[2]
	if len(positional) > 3 {
		args.Notes = positional[3]
	}

	switch args.Status {
	case "pass", "fail", "skip":
	default:
		return args, fmt.Errorf("verification status must be one of pass, fail, skip")
	}

	return args, nil
}

func RunVerifyStep(argv []string, stdout, stderr io.Writer) int {
	args, err := ParseVerifyStepArgs(argv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify-step: %v\n", err)
		return 1
	}

	benchmarkDir := resolveBenchmarkDir()
	resultsDir := filepath.Join(benchmarkDir, "results")

	reportFile := args.ReportFile
	if reportFile == "" {
		reportFile = resolveReportFile(resultsDir, args.ReportType)
	}

	if reportFile == "" || !fileExists(reportFile) {
		_, _ = fmt.Fprintln(stderr, "ERROR: no benchmark report found")
		return 1
	}

	stepID := fmt.Sprintf("%d.%d", args.Group, args.Step)
	timestamp := time.Now().UTC().Format(time.RFC3339)

	report, err := os.ReadFile(reportFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ERROR: failed to read report: %v\n", err)
		return 1
	}

	var data map[string]any
	if err := json.Unmarshal(report, &data); err != nil {
		_, _ = fmt.Fprintf(stderr, "ERROR: failed to parse report: %v\n", err)
		return 1
	}

	steps, ok := data["steps"].([]any)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "ERROR: report has no steps array")
		return 1
	}

	stepFound := false
	stepIsAnswer := false
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if step["id"] == stepID {
			stepFound = true
			if step["status"] == "answer" {
				stepIsAnswer = true
			}
			break
		}
	}

	if !stepFound {
		_, _ = fmt.Fprintf(stderr, "ERROR: step not found: %s\n", stepID)
		return 1
	}
	if !stepIsAnswer {
		_, _ = fmt.Fprintf(stderr, "ERROR: step is not answer-status and cannot be verified: %s\n", stepID)
		return 1
	}

	for i, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if step["id"] == stepID {
			step["verification"] = map[string]any{
				"status":    args.Status,
				"notes":     args.Notes,
				"timestamp": timestamp,
			}
			steps[i] = step
			break
		}
	}
	data["steps"] = steps

	updateVerificationTotals(data)

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ERROR: failed to marshal report: %v\n", err)
		return 1
	}

	if err := os.WriteFile(reportFile, output, 0644); err != nil {
		_, _ = fmt.Fprintf(stderr, "ERROR: failed to write report: %v\n", err)
		return 1
	}

	if args.Status == "fail" {
		errLog := filepath.Join(resultsDir, "errors.log")
		f, err := os.OpenFile(errLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "[%s] Step %s VERIFICATION FAILED: %s\n", timestamp, stepID, args.Notes)
			_ = f.Close()
		}
	}

	_, _ = fmt.Fprintf(stdout, "Verified: Step %s = %s\n", stepID, args.Status)
	return 0
}

func updateVerificationTotals(data map[string]any) {
	steps, _ := data["steps"].([]any)
	totals, ok := data["totals"].(map[string]any)
	if !ok {
		totals = make(map[string]any)
		data["totals"] = totals
	}

	var passed, failed, skipped, pending int
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if step["status"] != "answer" {
			continue
		}
		verification, _ := step["verification"].(map[string]any)
		vstatus, _ := verification["status"].(string)
		switch vstatus {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skip":
			skipped++
		case "pending", "":
			pending++
		}
	}

	totals["steps_verified_passed"] = passed
	totals["steps_verified_failed"] = failed
	totals["steps_verified_skipped"] = skipped
	totals["steps_pending_verification"] = pending
}

func resolveReportFile(resultsDir, reportType string) string {
	path, _ := resolveActiveReport(resultsDir, reportType)
	return path
}

// resolveActiveReport returns (reportFile, reportType). If reportType is provided,
// it looks for that specific type. Otherwise it auto-detects from pointer files.
func resolveActiveReport(resultsDir, reportType string) (string, string) {
	pointers := map[string]string{
		"baseline":      "current_baseline_report.txt",
		"pinchtab":      "current_pinchtab_report.txt",
		"agent":         "current_pinchtab_report.txt",
		"agent-browser": "current_agent_browser_report.txt",
		"agent_browser": "current_agent_browser_report.txt",
	}

	fallbacks := map[string]string{
		"baseline":      "baseline_*.json",
		"pinchtab":      "pinchtab_benchmark_*.json",
		"agent":         "pinchtab_benchmark_*.json",
		"agent-browser": "agent_browser_benchmark_*.json",
		"agent_browser": "agent_browser_benchmark_*.json",
	}

	// Normalize type aliases
	normalizeType := func(t string) string {
		if t == "agent" {
			return "pinchtab"
		}
		if t == "agent_browser" {
			return "agent-browser"
		}
		return t
	}

	if reportType != "" {
		ptr, ok := pointers[reportType]
		if !ok {
			return "", ""
		}
		if path := readPointerFile(filepath.Join(resultsDir, ptr)); path != "" {
			return path, normalizeType(reportType)
		}
		pattern := fallbacks[reportType]
		if path := mostRecentMatch(filepath.Join(resultsDir, pattern)); path != "" {
			return path, normalizeType(reportType)
		}
		return "", ""
	}

	// Auto-detect: use the most recently modified pointer file
	order := []string{"agent-browser", "pinchtab", "baseline"}
	var bestType string
	var bestPath string
	var bestTime time.Time
	for _, t := range order {
		ptrPath := filepath.Join(resultsDir, pointers[t])
		info, err := os.Stat(ptrPath)
		if err != nil {
			continue
		}
		if path := readPointerFile(ptrPath); path != "" {
			if bestPath == "" || info.ModTime().After(bestTime) {
				bestType = t
				bestPath = path
				bestTime = info.ModTime()
			}
		}
	}
	if bestPath != "" {
		return bestPath, bestType
	}

	// Fallback to pattern matching
	typeForPattern := map[string]string{
		"agent_browser_benchmark_*.json": "agent-browser",
		"pinchtab_benchmark_*.json":      "pinchtab",
		"baseline_*.json":                "baseline",
	}
	patterns := []string{
		"agent_browser_benchmark_*.json",
		"pinchtab_benchmark_*.json",
		"baseline_*.json",
	}
	for _, p := range patterns {
		if path := mostRecentMatch(filepath.Join(resultsDir, p)); path != "" {
			return path, typeForPattern[p]
		}
	}

	return "", ""
}

func readPointerFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func mostRecentMatch(pattern string) string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	var newest string
	var newestTime time.Time
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = m
			newestTime = info.ModTime()
		}
	}
	return newest
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
