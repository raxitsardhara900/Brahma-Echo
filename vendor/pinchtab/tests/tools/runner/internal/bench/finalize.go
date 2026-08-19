package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func runFinalize(reportFile string, stdout, stderr io.Writer) {
	data, err := os.ReadFile(reportFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "[finalize] cannot read %s: %v\n", reportFile, err)
		return
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		_, _ = fmt.Fprintf(stderr, "[finalize] cannot parse %s: %v\n", reportFile, err)
		return
	}

	totals, _ := report["totals"].(map[string]any)
	getF := func(key string) float64 {
		if v, ok := totals[key].(float64); ok {
			return v
		}
		return 0
	}

	usage, _ := report["run_usage"].(map[string]any)
	getU := func(key string) float64 {
		if usage == nil {
			return 0
		}
		if v, ok := usage[key].(float64); ok {
			return v
		}
		return 0
	}

	bench, _ := report["benchmark"].(map[string]any)
	benchType, _ := bench["type"].(string)
	model, _ := bench["model"].(string)

	answered := getF("steps_answered")
	execFailed := getF("steps_failed")
	execSkipped := getF("steps_skipped")
	legacyPassed := getF("steps_passed")
	verifiedPassed := getF("steps_verified_passed")
	verifiedFailed := getF("steps_verified_failed")
	verifiedSkipped := getF("steps_verified_skipped")
	pending := getF("steps_pending_verification")
	toolCalls := getF("tool_calls")

	isLegacyBaseline := benchType == "baseline" && answered == 0
	legacyTotal := legacyPassed + execFailed + execSkipped
	execTotal := answered + execFailed + execSkipped

	pct := func(a, b float64) string {
		if b == 0 {
			return "0.0%"
		}
		return fmt.Sprintf("%.1f%%", (a/b)*100)
	}

	var sb strings.Builder
	sb.WriteString("# Benchmark Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	fmt.Fprintf(&sb, "| Type | %s |\n", benchType)
	fmt.Fprintf(&sb, "| Model | %s |\n", model)

	if isLegacyBaseline {
		fmt.Fprintf(&sb, "| Steps Passed | %.0f |\n", legacyPassed)
		fmt.Fprintf(&sb, "| Steps Failed | %.0f |\n", execFailed)
		fmt.Fprintf(&sb, "| Steps Skipped | %.0f |\n", execSkipped)
		fmt.Fprintf(&sb, "| Pass Rate | %s |\n", pct(legacyPassed, legacyTotal))
		fmt.Fprintf(&sb, "| Tool Calls | %.0f |\n", toolCalls)
	} else {
		fmt.Fprintf(&sb, "| Steps Answered | %.0f |\n", answered)
		fmt.Fprintf(&sb, "| Execution Failed | %.0f |\n", execFailed)
		fmt.Fprintf(&sb, "| Execution Skipped | %.0f |\n", execSkipped)
		fmt.Fprintf(&sb, "| Answer Rate | %s |\n", pct(answered, execTotal))
		fmt.Fprintf(&sb, "| Verified Passed | %.0f |\n", verifiedPassed)
		fmt.Fprintf(&sb, "| Verified Failed | %.0f |\n", verifiedFailed)
		fmt.Fprintf(&sb, "| Verified Skipped | %.0f |\n", verifiedSkipped)
		fmt.Fprintf(&sb, "| Pending Verification | %.0f |\n", pending)
		fmt.Fprintf(&sb, "| Verification Pass Rate | %s |\n", pct(verifiedPassed, execTotal))
		fmt.Fprintf(&sb, "| Tool Calls | %.0f |\n", toolCalls)
	}

	sb.WriteString("\n## Run Usage\n\n")
	if usage == nil || (getU("total_tokens") == 0 && getU("request_count") == 0) {
		sb.WriteString("- none recorded\n")
	} else {
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")
		source, _ := usage["source"].(string)
		provider, _ := usage["provider"].(string)
		if source == "" {
			source = "unknown"
		}
		if provider == "" {
			provider = "unknown"
		}
		fmt.Fprintf(&sb, "| Source | %s |\n", source)
		fmt.Fprintf(&sb, "| Provider | %s |\n", provider)
		fmt.Fprintf(&sb, "| API Requests | %.0f |\n", getU("request_count"))
		fmt.Fprintf(&sb, "| Input Tokens (uncached) | %.0f |\n", getU("input_tokens"))
		fmt.Fprintf(&sb, "| Cache Creation Input Tokens | %.0f |\n", getU("cache_creation_input_tokens"))
		fmt.Fprintf(&sb, "| Cache Read Input Tokens | %.0f |\n", getU("cache_read_input_tokens"))
		fmt.Fprintf(&sb, "| Total Input Tokens | %.0f |\n", getU("total_input_tokens"))
		fmt.Fprintf(&sb, "| Output Tokens | %.0f |\n", getU("output_tokens"))
		fmt.Fprintf(&sb, "| Total Tokens | %.0f |\n", getU("total_tokens"))
	}

	steps, _ := report["steps"].([]any)

	sb.WriteString("\n## Pending Verification\n\n")
	var hasPending bool
	for _, s := range steps {
		step, _ := s.(map[string]any)
		status, _ := step["status"].(string)
		ver, _ := step["verification"].(map[string]any)
		verStatus, _ := ver["status"].(string)
		if verStatus == "" {
			verStatus = "pending"
		}
		if status == "answer" && verStatus == "pending" {
			id, _ := step["id"].(string)
			answer, _ := step["answer"].(string)
			if answer == "" {
				answer, _ = step["notes"].(string)
			}
			fmt.Fprintf(&sb, "- %s: %s\n", id, answer)
			hasPending = true
		}
	}
	if !hasPending {
		sb.WriteString("- none\n")
	}

	sb.WriteString("\n## Failed Steps\n\n")
	var hasFailed bool
	for _, s := range steps {
		step, _ := s.(map[string]any)
		status, _ := step["status"].(string)
		if status == "fail" {
			id, _ := step["id"].(string)
			notes, _ := step["notes"].(string)
			fmt.Fprintf(&sb, "- %s: %s\n", id, notes)
			hasFailed = true
		}
	}
	if !hasFailed {
		sb.WriteString("- none\n")
	}

	sb.WriteString("\n## Verification Failures\n\n")
	var hasVerFail bool
	for _, s := range steps {
		step, _ := s.(map[string]any)
		status, _ := step["status"].(string)
		ver, _ := step["verification"].(map[string]any)
		verStatus, _ := ver["status"].(string)
		if status == "answer" && verStatus == "fail" {
			id, _ := step["id"].(string)
			notes, _ := ver["notes"].(string)
			fmt.Fprintf(&sb, "- %s: %s\n", id, notes)
			hasVerFail = true
		}
	}
	if !hasVerFail {
		sb.WriteString("- none\n")
	}

	summaryFile := strings.TrimSuffix(reportFile, ".json") + "_summary.md"
	if err := os.WriteFile(summaryFile, []byte(sb.String()), 0644); err != nil {
		_, _ = fmt.Fprintf(stderr, "[finalize] cannot write %s: %v\n", summaryFile, err)
		return
	}

	_, _ = fmt.Fprintf(stdout, "Wrote %s\n\n", summaryFile)
	_, _ = fmt.Fprint(stdout, sb.String())
}
