package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestNavAutoStartsBeforeNavigateAndSnap(t *testing.T) {
	var paths []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/health":
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		case "/navigate":
			_, _ = io.WriteString(w, `{"tabId":"T1","status":"ok"}`)
		case "/snapshot":
			_, _ = io.WriteString(w, `{"nodes":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	oldServerURL := serverURL
	serverURL = ts.URL
	defer func() { serverURL = oldServerURL }()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	flags := navCmd.Flags()
	oldTab, _ := flags.GetString("tab")
	oldSnap, _ := flags.GetBool("snap")
	defer func() {
		_ = flags.Set("tab", oldTab)
		_ = flags.Set("snap", strconv.FormatBool(oldSnap))
	}()
	_ = flags.Set("tab", "")
	_ = flags.Set("snap", "true")

	_ = captureStdout(t, func() {
		navCmd.Run(navCmd, []string{"example.com"})
	})

	if len(paths) < 3 || paths[0] != "/health" || paths[1] != "/navigate" || paths[2] != "/snapshot" {
		t.Fatalf("request paths = %v, want /health, /navigate, /snapshot", paths)
	}
}

func TestSnapDoesNotAutoStartAndTreatsArgAsSelector(t *testing.T) {
	var paths []string
	var selector string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/snapshot":
			selector = r.URL.Query().Get("selector")
			_, _ = io.WriteString(w, `{"nodes":[]}`)
		case "/health":
			t.Error("snap should not check health or auto-start")
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	oldServerURL := serverURL
	serverURL = ts.URL
	defer func() { serverURL = oldServerURL }()

	flags := snapCmd.Flags()
	oldSelector, _ := flags.GetString("selector")
	defer func() {
		_ = flags.Set("selector", oldSelector)
	}()
	_ = flags.Set("selector", "")

	_ = captureStdout(t, func() {
		snapCmd.Run(snapCmd, []string{"#main"})
	})

	if selector != "#main" {
		t.Fatalf("snapshot selector = %q, want #main", selector)
	}
	if got, _ := flags.GetString("selector"); got != "" {
		t.Fatalf("snap selector flag = %q, want unchanged empty flag", got)
	}
	if len(paths) != 1 || paths[0] != "/snapshot" {
		t.Fatalf("request paths = %v, want only /snapshot", paths)
	}
}
