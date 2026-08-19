package opt

import (
	"fmt"
	"io"
)

func Run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: runner opt <merge-reports|verify-answers> [args...]")
		return 1
	}

	switch argv[0] {
	case "merge-reports":
		return RunMergeReports(argv[1:], stdout, stderr)
	case "verify-answers":
		return RunVerifyAnswers(argv[1:], stdout, stderr)
	case "inject-usage":
		return RunInjectUsage(argv[1:], stdout, stderr)
	case "summarize":
		return RunSummarize(argv[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "runner opt: unknown subcommand %q\n", argv[0])
		_, _ = fmt.Fprintln(stderr, "Use one of: merge-reports, verify-answers, inject-usage, summarize")
		return 1
	}
}
