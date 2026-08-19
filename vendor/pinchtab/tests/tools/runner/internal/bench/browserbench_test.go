package bench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBrowserBenchArgs(t *testing.T) {
	args, err := parseBrowserBenchArgs([]string{"--provider", "openai", "--tasks", "5", "--task-id", "12", "--verbose"})
	if err != nil {
		t.Fatalf("parseBrowserBenchArgs returned error: %v", err)
	}
	if args.Provider != ProviderOpenAI {
		t.Fatalf("Provider = %q, want %q", args.Provider, ProviderOpenAI)
	}
	if args.Tasks != 5 {
		t.Fatalf("Tasks = %d, want 5", args.Tasks)
	}
	if args.TaskID != "12" {
		t.Fatalf("TaskID = %q, want 12", args.TaskID)
	}
	if !args.Verbose {
		t.Fatal("Verbose = false, want true")
	}
}

func TestSelectBrowserBenchTasks(t *testing.T) {
	tasks := []BrowserBenchTask{{TaskID: "1"}, {TaskID: "2"}, {TaskID: "3"}}
	selected := selectBrowserBenchTasks(tasks, "2", 0)
	if len(selected) != 1 || selected[0].TaskID != "2" {
		t.Fatalf("selected = %#v, want only task 2", selected)
	}

	selected = selectBrowserBenchTasks(tasks, "", 2)
	if len(selected) != 2 || selected[1].TaskID != "2" {
		t.Fatalf("limit selection = %#v, want first two tasks", selected)
	}
}

func TestEvaluateBrowserBenchAnswer(t *testing.T) {
	cases := []struct {
		answer string
		truth  string
		want   bool
	}{
		{"FINAL_ANSWER: 3.4", "3.4", true},
		{"yes", "Yes", true},
		{"The answer is Without Me", "Without Me", true},
		{"No", "Yes", false},
	}
	for _, tc := range cases {
		if got := evaluateBrowserBenchAnswer(extractFinalAnswer(tc.answer), tc.truth); got != tc.want {
			t.Fatalf("evaluateBrowserBenchAnswer(%q, %q) = %v, want %v", tc.answer, tc.truth, got, tc.want)
		}
	}
}

func TestDeriveSolveMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "commands.ndjson")
	content := `{"output":"{\"solver\":\"cloudflare\",\"attempts\":2}"}
{"output":"{\"solver\":\"semantic\",\"attempts\":1}"}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	solver, attempts := deriveSolveMetadata(path)
	if solver != "semantic" {
		t.Fatalf("solver = %q, want semantic", solver)
	}
	if attempts != "2" {
		t.Fatalf("attempts = %q, want 2", attempts)
	}
}
