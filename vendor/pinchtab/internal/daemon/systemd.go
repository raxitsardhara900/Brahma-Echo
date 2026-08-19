package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type systemdUserManager struct {
	env    environment
	runner commandRunner
}

func (m *systemdUserManager) ServicePath() string {
	return filepath.Join(systemdUserConfigHome(m.env), "systemd", "user", pinchtabDaemonUnitName)
}

func (m *systemdUserManager) Preflight() error {
	if _, err := runCommand(m.runner, "systemctl", "--user", "show-environment"); err != nil {
		return fmt.Errorf("linux daemon install requires a working user systemd session (`systemctl --user`): %w", err)
	}
	return nil
}

func (m *systemdUserManager) Install(configPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(m.ServicePath()), 0755); err != nil {
		return "", fmt.Errorf("create systemd user directory: %w", err)
	}
	if err := ensureDaemonLogDir(m.env); err != nil {
		return "", err
	}
	if err := os.WriteFile(m.ServicePath(), []byte(renderSystemdUnit(m.env.execPath, configPath, daemonStdoutLogPath(m.env), daemonStderrLogPath(m.env))), 0644); err != nil {
		return "", fmt.Errorf("write systemd unit: %w", err)
	}
	if _, err := runCommand(m.runner, "systemctl", "--user", "daemon-reload"); err != nil {
		return "", err
	}
	if _, err := runCommand(m.runner, "systemctl", "--user", "enable", "--now", pinchtabDaemonUnitName); err != nil {
		return "", err
	}
	return fmt.Sprintf("Installed systemd user service at %s", m.ServicePath()), nil
}

func (m *systemdUserManager) Start() (string, error) {
	if err := ensureDaemonLogDir(m.env); err != nil {
		return "", err
	}
	if _, err := runCommand(m.runner, "systemctl", "--user", "start", pinchtabDaemonUnitName); err != nil {
		return "", err
	}
	return "Pinchtab daemon started.", nil
}

func (m *systemdUserManager) Restart() (string, error) {
	if err := ensureDaemonLogDir(m.env); err != nil {
		return "", err
	}
	if _, err := runCommand(m.runner, "systemctl", "--user", "restart", pinchtabDaemonUnitName); err != nil {
		return "", err
	}
	return "Pinchtab daemon restarted.", nil
}

func (m *systemdUserManager) Stop() (string, error) {
	if _, err := runCommand(m.runner, "systemctl", "--user", "stop", pinchtabDaemonUnitName); err != nil {
		return "", err
	}
	return "Pinchtab daemon stopped.", nil
}

func (m *systemdUserManager) Status() (string, error) {
	output, err := runCommand(m.runner, "systemctl", "--user", "status", pinchtabDaemonUnitName, "--no-pager")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" {
		return "Pinchtab daemon status returned no output.", nil
	}
	return output, nil
}

func (m *systemdUserManager) Uninstall() (string, error) {
	var errs []error
	if _, err := runCommand(m.runner, "systemctl", "--user", "disable", "--now", pinchtabDaemonUnitName); err != nil {
		errs = append(errs, err)
	}
	if err := os.Remove(m.ServicePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove unit file: %w", err))
	}
	if _, err := runCommand(m.runner, "systemctl", "--user", "daemon-reload"); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return "", errors.Join(errs...)
	}
	return "Pinchtab daemon uninstalled.", nil
}

func (m *systemdUserManager) Pid() (string, error) {
	output, err := runCommand(m.runner, "systemctl", "--user", "show", pinchtabDaemonUnitName, "--property", "MainPID")
	if err != nil {
		return "", err
	}
	if parts := strings.Split(output, "="); len(parts) == 2 {
		pid := strings.TrimSpace(parts[1])
		if pid == "0" {
			return "", nil
		}
		return pid, nil
	}
	return "", nil
}

func (m *systemdUserManager) Logs(n int) (string, error) {
	logPath := daemonStderrLogPath(m.env)
	if info, err := os.Stat(logPath); err == nil && info.Size() > 0 {
		return runCommand(m.runner, "tail", "-n", fmt.Sprintf("%d", n), logPath)
	}

	return runCommand(m.runner, "journalctl", "--user", "-u", pinchtabDaemonUnitName, "-n", fmt.Sprintf("%d", n), "--no-pager")
}

func (m *systemdUserManager) ManualInstructions() string {
	path := m.ServicePath()
	var b strings.Builder
	fmt.Fprintln(&b, "Manual instructions (Linux/systemd):")
	fmt.Fprintln(&b, "To install manually:")
	fmt.Fprintf(&b, "  1. Create %s\n", path)
	fmt.Fprintln(&b, "  2. Run: systemctl --user daemon-reload")
	fmt.Fprintln(&b, "  3. Run: systemctl --user enable --now pinchtab.service")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "To uninstall manually:")
	fmt.Fprintln(&b, "  1. Run: systemctl --user disable --now pinchtab.service")
	fmt.Fprintf(&b, "  2. Remove: %s\n", path)
	fmt.Fprintln(&b, "  3. Run: systemctl --user daemon-reload")
	return b.String()
}

// unitValue escapes a value for a systemd unit file. systemd expands "%" as a
// specifier prefix (%h, %i, ...), so a literal percent in a path must be
// doubled or the daemon starts against a mangled location.
func unitValue(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
}

func renderSystemdUnit(execPath, configPath, stdoutPath, stderrPath string) string {
	execPath = unitValue(execPath)
	configPath = unitValue(configPath)
	stdoutPath = unitValue(stdoutPath)
	stderrPath = unitValue(stderrPath)
	return fmt.Sprintf(`[Unit]
Description=Pinchtab Browser Service
After=network.target

[Service]
Type=simple
ExecStart="%s" server
Environment="PINCHTAB_CONFIG=%s"
StandardOutput=append:%s
StandardError=append:%s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, execPath, configPath, stdoutPath, stderrPath)
}
