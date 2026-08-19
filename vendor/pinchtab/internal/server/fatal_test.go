package server

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fatalRun struct {
	stderr string
	codes  []int
}

func runFatal(t *testing.T, fn func()) fatalRun {
	t.Helper()

	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(new(bytes.Buffer), nil)))
	origExit := exitProcess
	var codes []int
	exitProcess = func(code int) { codes = append(codes, code) }
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		exitProcess = origExit
		slog.SetDefault(origLogger)
	})

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		close(done)
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer error = %v", err)
	}
	<-done
	os.Stderr = origStderr
	return fatalRun{stderr: buf.String(), codes: codes}
}

func TestFatalStartupReportsOnStderrWhileLoggerIsDiscarded(t *testing.T) {
	run := runFatal(t, func() {
		slog.Error("swallowed by the discard handler", "err", "invisible")
		fatalStartup("activity store", errors.New("open ledger: permission denied"))
	})

	for _, want := range []string{"activity store", "open ledger: permission denied"} {
		if !strings.Contains(run.stderr, want) {
			t.Fatalf("stderr %q does not contain %q", run.stderr, want)
		}
	}
	if strings.Contains(run.stderr, "swallowed by the discard handler") {
		t.Fatalf("discarded logger still reached stderr: %q", run.stderr)
	}
	if len(run.codes) != 1 || run.codes[0] != 1 {
		t.Fatalf("exit codes = %v, want [1]", run.codes)
	}
}

func TestFatalStartupPortInUseNamesAddressAndRemedy(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer func() { _ = held.Close() }()
	addr := held.Addr().String()

	_, listenErr := net.Listen("tcp", addr)
	if listenErr == nil {
		t.Fatal("expected the second listen on a held address to fail")
	}

	run := runFatal(t, func() {
		fatalStartup("cannot listen on "+addr, listenErr)
	})

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("net.SplitHostPort() error = %v", err)
	}
	for _, want := range []string{port, "already listening", "pinchtab server stop", "pinchtab daemon"} {
		if !strings.Contains(run.stderr, want) {
			t.Fatalf("stderr %q does not contain %q", run.stderr, want)
		}
	}
}

func TestStartupFatalHintsOnlyForAddressInUse(t *testing.T) {
	if hints := startupFatalHints(errors.New("open ledger: permission denied")); len(hints) != 0 {
		t.Fatalf("unrelated error produced hints: %v", hints)
	}
}

// The discard handler can silence any slog-based fatal, so every log-then-exit
// pair must funnel through fatalStartup; this guards the funnel from a new
// call site re-introducing a direct exit.
func TestFatalStartupIsTheOnlyExitOneSite(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	scanned := 0
	var offenders []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", file, err)
		}
		scanned++
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "os.Exit(1)") && !strings.Contains(line, "exitProcess(1)") {
				continue
			}
			if file == "server.go" && insideFatalStartup(string(src), i) {
				continue
			}
			offenders = append(offenders, file+":"+line)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no package sources")
	}
	if len(offenders) != 0 {
		t.Fatalf("exit-1 outside fatalStartup: %v", offenders)
	}
	if !insideFatalStartupContainsExit() {
		t.Fatal("fatalStartup no longer exits with code 1")
	}
}

func insideFatalStartup(src string, line int) bool {
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "func fatalStartup(") {
			start = i
			continue
		}
		if start >= 0 && l == "}" {
			return line > start && line < i
		}
	}
	return false
}

func insideFatalStartupContainsExit() bool {
	src, err := os.ReadFile("server.go")
	if err != nil {
		return false
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, "exitProcess(1)") && insideFatalStartup(string(src), i) {
			return true
		}
	}
	return false
}

// "dashboard started" must follow a successful bind; the bind is the step that
// fails when the port is taken, so announcing it first misreports the failure.
func TestDashboardStartedIsLoggedAfterBind(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	bind := strings.Index(string(src), `net.Listen("tcp", srv.Addr)`)
	started := strings.Index(string(src), `slog.Info("dashboard started"`)
	serve := strings.Index(string(src), "srv.Serve(listener)")
	if bind < 0 || started < 0 || serve < 0 {
		t.Fatalf("missing bind/started/serve markers: bind=%d started=%d serve=%d", bind, started, serve)
	}
	if bind >= started || started >= serve {
		t.Fatalf("expected bind < started < serve, got bind=%d started=%d serve=%d", bind, started, serve)
	}
}
