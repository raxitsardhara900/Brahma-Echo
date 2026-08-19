package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/handlers"
	"github.com/pinchtab/pinchtab/internal/safelog"
)

// installCapturedDefault captures the process logger through the very handler
// InstallDefault builds, so these tests observe what a real run records.
func installCapturedDefault(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(safelog.NewDefaultHandler(&buf)))
	safelog.SetLevel(safelog.DefaultLevel)
	t.Cleanup(func() {
		slog.SetDefault(previous)
		safelog.SetLevel(safelog.DefaultLevel)
	})
	return &buf
}

func TestDefaultRunRecordsTheRequestLine(t *testing.T) {
	buf := installCapturedDefault(t)

	handler := handlers.LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "b5fc54c8642370c7")
		w.WriteHeader(http.StatusNotFound)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/click", nil))

	out := buf.String()
	for _, want := range []string{"msg=request", "requestId=b5fc54c8642370c7", "method=POST", "path=/click", "status=404"} {
		if !strings.Contains(out, want) {
			t.Errorf("a default run dropped %q from the access log:\n%s", want, out)
		}
	}
}

func TestDefaultRunRecordsErrorsAndWarnings(t *testing.T) {
	buf := installCapturedDefault(t)

	slog.Warn("always-on: instance stopped deliberately", "id", "inst_ccb22b39")
	slog.Error("🔥 TARGET CRASHED", "target", "page")

	out := buf.String()
	if !strings.Contains(out, "instance stopped deliberately") {
		t.Errorf("a default run dropped a warning:\n%s", out)
	}
	if !strings.Contains(out, "TARGET CRASHED") {
		t.Errorf("a default run dropped an error:\n%s", out)
	}
}
