package bench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// stepHeaderRegex matches lines like "### 0.1 Navigate to page" in a
// group-*.md file. The group number is the first capture, the
// step number is the second, and the remainder is the human-readable title.
var stepHeaderRegex = regexp.MustCompile(`^###\s+(\d+)\.(\d+)\s+(.*)$`)

// groupStep is one row in the start-of-run summary: a single step within a
// group, rendered as "  0.1  Navigate to page".
type groupStep struct {
	Group int
	Step  int
	Title string
}

// loadGroupSteps reads a single group-NN.md file and returns the
// step headers found inside. Returns nil on any read/parse failure — callers
// treat a missing file as an empty group rather than fatal, because we don't
// want the summary print to ever block or crash the actual benchmark run.
func loadGroupSteps(benchmarkDir string, group int) []groupStep {
	path := filepath.Join(benchmarkDir, fmt.Sprintf("group-%02d.md", group))
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var steps []groupStep
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := stepHeaderRegex.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		var g, s int
		_, _ = fmt.Sscanf(m[1], "%d", &g)
		_, _ = fmt.Sscanf(m[2], "%d", &s)
		steps = append(steps, groupStep{
			Group: g,
			Step:  s,
			Title: strings.TrimSpace(m[3]),
		})
	}
	return steps
}

// listGroups returns the list of group numbers to summarize. When the caller
// passes an explicit subset (--groups or --profile), we use that; otherwise we
// scan the benchmark directory for all group-NN.md files.
func listGroups(benchmarkDir string, explicit []int) []int {
	if len(explicit) > 0 {
		out := append([]int(nil), explicit...)
		sort.Ints(out)
		return out
	}
	entries, err := os.ReadDir(benchmarkDir)
	if err != nil {
		return nil
	}
	var groups []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if m := groupFileRegex.FindStringSubmatch(e.Name()); m != nil {
			var n int
			_, _ = fmt.Sscanf(m[1], "%d", &n)
			groups = append(groups, n)
		}
	}
	sort.Ints(groups)
	return groups
}

// PrintStartBanner prints the harness-generated "here's what we're about to
// run" summary before the agent loop starts. This is the summary the user
// asked to always see — agent-browser group 0 is synthesized in
// prompt.go::agentBrowserGroup0(), so we inline the same steps here rather
// than try to parse that Go-string.
func PrintStartBanner(w io.Writer, lane Lane, benchmarkDir string, groups []int, model, reportFile string) {
	selected := listGroups(benchmarkDir, groups)

	_, _ = fmt.Fprintf(w, "\n%s══ Benchmark start ══%s\n", colorBold, colorReset)
	_, _ = fmt.Fprintf(w, "  lane:   %s\n", lane)
	_, _ = fmt.Fprintf(w, "  model:  %s\n", model)
	_, _ = fmt.Fprintf(w, "  report: %s\n", reportFile)

	if len(selected) == 0 {
		_, _ = fmt.Fprintf(w, "  (no groups selected)\n")
		return
	}

	totalSteps := 0
	for _, g := range selected {
		steps := loadGroupSteps(benchmarkDir, g)
		_, _ = fmt.Fprintf(w, "\n  Group %d — %d step(s):\n", g, len(steps))
		for _, s := range steps {
			_, _ = fmt.Fprintf(w, "    %d.%d  %s\n", s.Group, s.Step, s.Title)
		}
		totalSteps += len(steps)
	}
	_, _ = fmt.Fprintf(w, "\n  Total: %d step(s) across %d group(s)\n", totalSteps, len(selected))
	_, _ = fmt.Fprintln(w)
}

// reportSummary is a trimmed-down view of the benchmark JSON report —
// everything we need to print a pass/fail breakdown at end-of-run without
// re-invoking jq.
type reportSummary struct {
	Totals struct {
		StepsAnswered            int `json:"steps_answered"`
		StepsFailed              int `json:"steps_failed"`
		StepsSkipped             int `json:"steps_skipped"`
		StepsVerifiedPassed      int `json:"steps_verified_passed"`
		StepsVerifiedFailed      int `json:"steps_verified_failed"`
		StepsVerifiedSkipped     int `json:"steps_verified_skipped"`
		StepsPendingVerification int `json:"steps_pending_verification"`
		ToolCalls                int `json:"tool_calls"`
	} `json:"totals"`
	Steps []struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Notes        string `json:"notes"`
		Answer       string `json:"answer"`
		Verification struct {
			Status string `json:"status"`
			Notes  string `json:"notes"`
		} `json:"verification"`
	} `json:"steps"`
}

// PrintEndBanner prints a compact pass/fail table at end-of-run.
func PrintEndBanner(w io.Writer, reportFile string) {
	data, err := os.ReadFile(reportFile)
	if err != nil {
		_, _ = fmt.Fprintf(w, "\n%s[end] could not read report %s: %v%s\n", colorYellow, reportFile, err, colorReset)
		return
	}
	var r reportSummary
	if err := json.Unmarshal(data, &r); err != nil {
		_, _ = fmt.Fprintf(w, "\n%s[end] could not parse report %s: %v%s\n", colorYellow, reportFile, err, colorReset)
		return
	}

	total := r.Totals.StepsAnswered + r.Totals.StepsFailed + r.Totals.StepsSkipped
	_, _ = fmt.Fprintf(w, "\n%s══ Benchmark result ══%s\n", colorBold, colorReset)
	_, _ = fmt.Fprintf(w, "  answered:             %d\n", r.Totals.StepsAnswered)
	_, _ = fmt.Fprintf(w, "  execution failed:     %d\n", r.Totals.StepsFailed)
	_, _ = fmt.Fprintf(w, "  execution skipped:    %d\n", r.Totals.StepsSkipped)
	_, _ = fmt.Fprintf(w, "  verified passed:      %s%d%s\n", colorGreen, r.Totals.StepsVerifiedPassed, colorReset)
	_, _ = fmt.Fprintf(w, "  verified failed:      %s%d%s\n", colorYellow, r.Totals.StepsVerifiedFailed, colorReset)
	_, _ = fmt.Fprintf(w, "  verified skipped:     %d\n", r.Totals.StepsVerifiedSkipped)
	_, _ = fmt.Fprintf(w, "  pending verification: %d\n", r.Totals.StepsPendingVerification)
	_, _ = fmt.Fprintf(w, "  tool calls:           %d\n", r.Totals.ToolCalls)
	if total > 0 {
		passRate := float64(r.Totals.StepsVerifiedPassed*1000/total) / 10.0
		_, _ = fmt.Fprintf(w, "  verification pass rate: %.1f%% (%d/%d)\n",
			passRate, r.Totals.StepsVerifiedPassed, total)
	}

	// List failures inline so the user doesn't need to open _summary.md for
	// the "what went wrong" info. Keep it terse — one line per step.
	var execFails, verFails []string
	for _, s := range r.Steps {
		if s.Status == "fail" {
			execFails = append(execFails, fmt.Sprintf("    %s: %s", s.ID, trimOneLine(s.Notes, 100)))
		}
		if s.Status == "answer" && s.Verification.Status == "fail" {
			verFails = append(verFails, fmt.Sprintf("    %s: %s", s.ID, trimOneLine(s.Verification.Notes, 100)))
		}
	}
	if len(execFails) > 0 {
		_, _ = fmt.Fprintf(w, "\n  %sExecution failures:%s\n", colorYellow, colorReset)
		for _, line := range execFails {
			_, _ = fmt.Fprintln(w, line)
		}
	}
	if len(verFails) > 0 {
		_, _ = fmt.Fprintf(w, "\n  %sVerification failures:%s\n", colorYellow, colorReset)
		for _, line := range verFails {
			_, _ = fmt.Fprintln(w, line)
		}
	}
	_, _ = fmt.Fprintln(w)
}

func trimOneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if max > 0 && len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
