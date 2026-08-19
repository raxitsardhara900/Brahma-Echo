package bench

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type StepEndArgs struct {
	ReportType   string
	ReportFile   string
	Group        int
	Step         int
	RecordStatus string
	AnswerText   string
	VerifyStatus string
	VerifyNotes  string
}

func ParseStepEndArgs(argv []string) (StepEndArgs, error) {
	var args StepEndArgs

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
	if len(positional) < 5 {
		return args, errors.New("usage: step-end [--type TYPE] [--report-file PATH] <group> <step> <answer|fail|skip> \"<answer-text>\" <pass|fail|skip> \"<verify-notes>\"")
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
	args.RecordStatus = positional[2]
	args.AnswerText = positional[3]
	args.VerifyStatus = positional[4]
	if len(positional) > 5 {
		args.VerifyNotes = positional[5]
	}

	switch args.RecordStatus {
	case "answer", "fail", "skip":
	default:
		return args, fmt.Errorf("record status must be one of answer, fail, skip")
	}

	switch args.VerifyStatus {
	case "pass", "fail", "skip":
	default:
		return args, fmt.Errorf("verify status must be one of pass, fail, skip")
	}

	return args, nil
}

func RunStepEnd(argv []string, stdout, stderr io.Writer) int {
	args, err := ParseStepEndArgs(argv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "step-end: %v\n", err)
		return 1
	}

	// Auto-resolve report file and type if not provided
	resultsDir := filepath.Join(resolveBenchmarkDir(), "results")
	if args.ReportFile == "" {
		reportFile, reportType := resolveActiveReport(resultsDir, args.ReportType)
		if reportFile == "" {
			if args.ReportType != "" {
				_, _ = fmt.Fprintf(stderr, "step-end: no active report found for type %q\n", args.ReportType)
			} else {
				_, _ = fmt.Fprintf(stderr, "step-end: no active benchmark report found\n")
			}
			return 1
		}
		args.ReportFile = reportFile
		if args.ReportType == "" {
			args.ReportType = reportType
		}
	}

	recordArgs := []string{}
	if args.ReportType != "" {
		recordArgs = append(recordArgs, "--type", args.ReportType)
	}
	if args.ReportFile != "" {
		recordArgs = append(recordArgs, "--report-file", args.ReportFile)
	}
	recordArgs = append(recordArgs,
		fmt.Sprintf("%d", args.Group),
		fmt.Sprintf("%d", args.Step),
		args.RecordStatus,
		args.AnswerText,
	)

	code := RunRecordStep(recordArgs, stdout, stderr)
	if code != 0 {
		return code
	}

	if args.RecordStatus == "answer" {
		verifyArgs := []string{}
		if args.ReportType != "" {
			verifyArgs = append(verifyArgs, "--type", args.ReportType)
		}
		if args.ReportFile != "" {
			verifyArgs = append(verifyArgs, "--report-file", args.ReportFile)
		}
		verifyArgs = append(verifyArgs,
			fmt.Sprintf("%d", args.Group),
			fmt.Sprintf("%d", args.Step),
			args.VerifyStatus,
			args.VerifyNotes,
		)

		code = RunVerifyStep(verifyArgs, stdout, stderr)
		if code != 0 {
			return code
		}
	}

	return 0
}
