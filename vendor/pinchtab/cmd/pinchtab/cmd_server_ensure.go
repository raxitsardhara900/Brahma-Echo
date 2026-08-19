package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pinchtab/pinchtab/internal/config"
	"github.com/pinchtab/pinchtab/internal/readiness"
	"github.com/pinchtab/pinchtab/internal/server"
)

const ensureServerTimeout = 30 * time.Second

type serverStartFunc func() error
type serverHealthFunc func(baseURL, token string) bool
type serverProbeFunc func(baseURL, token string) server.HealthProbe

func ensureServerForCLI(cfg *config.RuntimeConfig, baseURL, token, command string) error {
	return ensureServerWithAutoStart(baseURL, token, command, canAutoStartServerForCLI(cfg, baseURL), autoStartServer, probeServerHealth, ensureServerTimeout)
}

func ensureServerWith(baseURL, token, command string, start serverStartFunc, probe serverProbeFunc, timeout time.Duration) error {
	return ensureServerWithAutoStart(baseURL, token, command, true, start, probe, timeout)
}

func ensureServerWithAutoStart(baseURL, token, command string, allowAutoStart bool, start serverStartFunc, probe serverProbeFunc, timeout time.Duration) error {
	initial := probe(baseURL, token)
	if serverProbeHealthy(initial) {
		return nil
	}

	// Reachable but unhealthy: the server is up and answered this very probe, so
	// starting a second one cannot fix whatever it reported. Quote its reason
	// instead of waiting out the readiness timeout.
	if initial.Reachable {
		return fmt.Errorf("server at %s is running but unhealthy: %s", baseURL, unhealthyServerDetail(initial))
	}

	if !allowAutoStart {
		return fmt.Errorf("server at %s is not running; auto-start is only supported for the default local server", baseURL)
	}

	slog.Debug("server not running, starting automatically", "url", baseURL, "command", command)
	if err := start(); err != nil {
		slog.Error("failed to auto-start server", "err", err, "command", command)
		return fmt.Errorf("server at %s is not running and auto-start failed: %w", baseURL, err)
	}

	// The readiness wait keeps the bool semantics: a freshly spawned server
	// answers 503 while its browser comes up, which is not-ready-yet, not a
	// reason to abort.
	ready := func(url, tok string) bool { return serverProbeHealthy(probe(url, tok)) }
	if !waitForServerWith(baseURL, token, timeout, ready) {
		return fmt.Errorf("server did not become healthy at %s within %s", baseURL, timeout)
	}

	slog.Debug("server started successfully", "url", baseURL, "command", command)
	return nil
}

func unhealthyServerDetail(probe server.HealthProbe) string {
	if reason := probe.Reason(); reason != "" {
		return reason
	}
	return fmt.Sprintf("HTTP %d", probe.StatusCode)
}

func probeServerHealth(baseURL, token string) server.HealthProbe {
	return server.ProbeHealthWithToken(baseURL+"/health", 3*time.Second, token)
}

// serverProbeHealthy is the CLI ensure path's own readiness rule: anything the
// server answers below 500 means it is up enough to take the command.
func serverProbeHealthy(probe server.HealthProbe) bool {
	return probe.Reachable && probe.StatusCode < 500
}

func isServerHealthy(baseURL, token string) bool {
	return serverProbeHealthy(probeServerHealth(baseURL, token))
}

func autoStartServer() error {
	stateDir := stateDirForConfig(loadConfig())
	binary, marker, err := prepareServerSpawn()
	if err != nil {
		return err
	}

	args := autoStartServerArgs(marker)
	pid, err := spawnDetachedChild(binary, args, nil)
	if err != nil {
		return err
	}

	// Track the auto-started server's PID so `pinchtab server stop` can reap
	// it. Best-effort: failing to write the PID file is logged but not fatal.
	if err := recordServerPID(stateDir, pid, binary, args, "", marker); err != nil {
		slog.Warn("failed to write server pid file", "err", err)
	}

	return nil
}

func autoStartServerArgs(marker ...string) []string {
	args := []string{"server"}
	if len(marker) > 0 && strings.TrimSpace(marker[0]) != "" {
		args = append(args, "--background-child", marker[0])
	}
	return args
}

func waitForServer(baseURL, token string, timeout time.Duration) bool {
	return waitForServerWith(baseURL, token, timeout, isServerHealthy)
}

func waitForServerWith(baseURL, token string, timeout time.Duration, healthy serverHealthFunc) bool {
	_, err := readiness.WaitUntil(context.Background(), timeout, 500*time.Millisecond,
		func() (struct{}, bool, error) { return struct{}{}, healthy(baseURL, token), nil })
	return err == nil
}
