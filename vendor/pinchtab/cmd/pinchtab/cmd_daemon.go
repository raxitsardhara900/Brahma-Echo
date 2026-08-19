package main

import (
	"fmt"
	"os"

	"github.com/pinchtab/pinchtab/internal/browsers/chrome"
	"github.com/pinchtab/pinchtab/internal/browsers/runtimekit"
	"github.com/pinchtab/pinchtab/internal/cli"
	"github.com/pinchtab/pinchtab/internal/config"
	"github.com/pinchtab/pinchtab/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon [action]",
	Short: "Manage the background service",
	Long:  "Start, stop, install, or check the status of the PinchTab background service.",
	// The one leaf that reads a positional, so it declares its own arity rather than
	// inheriting the no-argument rule every other leaf gets. One at most: this dispatches
	// on args[0] and used to drop args[1:] silently, so `daemon status extra` reported the
	// status it was asked for and said nothing about the word it ignored.
	Args:          rejectExtraArguments,
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		jsonOut, _ := cmd.Flags().GetBool("json")
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}
		handleDaemonCommand(sub, jsonOut)
	},
}

func init() {
	daemonCmd.GroupID = "primary"
	daemonCmd.Flags().Bool("json", false, "Print daemon status as JSON (status only, no actions)")
	rootCmd.AddCommand(daemonCmd)
}

var daemonCurrentManager = daemon.CurrentManager

func handleDaemonCommand(subcommand string, jsonOut bool) {
	if code := dispatchDaemonCommand(subcommand, jsonOut); code != 0 {
		os.Exit(code)
	}
}

func dispatchDaemonCommand(subcommand string, jsonOut bool) int {
	if isDaemonStatusSubcommand(subcommand) {
		if jsonOut {
			printDaemonStatusJSON()
			return 0
		}
		printDaemonOverview()
		return 0
	}

	policy, declared := daemonNotInstalledPolicies[subcommand]
	if !declared {
		return printDaemonUsage(subcommand)
	}
	notInstalled, code, refused := applyDaemonNotInstalledPolicy(subcommand, policy)
	if refused {
		return code
	}

	manager, err := daemonCurrentManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.ErrorStyle, err.Error()))
		return 1
	}

	switch subcommand {
	case "install":
		handleDaemonInstall(manager)
	case "start":
		printDaemonManagerResult(manager.Start())
	case "restart":
		printDaemonManagerResult(manager.Restart())
	case "stop":
		handleDaemonStop(manager, notInstalled)
	case "uninstall":
		handleDaemonUninstall(manager, notInstalled)
	default:
		return printDaemonUsage(subcommand)
	}
	return 0
}

func printDaemonUsage(subcommand string) int {
	fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.ErrorStyle, fmt.Sprintf("unknown daemon command: %s", subcommand)))
	fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.MutedStyle, "Usage: pinchtab daemon <status|install|start|restart|stop|uninstall>"))
	return unknownSubcommandExitCode
}

func isDaemonStatusSubcommand(subcommand string) bool {
	switch subcommand {
	case "", "status", "help", "--help", "-h":
		return true
	}
	return false
}

type daemonNotInstalledPolicy int

const (
	daemonRefuse daemonNotInstalledPolicy = iota
	daemonNoOp
	daemonProceed
)

var daemonNotInstalledPolicies = map[string]daemonNotInstalledPolicy{
	"install":   daemonProceed,
	"start":     daemonRefuse,
	"restart":   daemonRefuse,
	"stop":      daemonNoOp,
	"uninstall": daemonNoOp,
}

var daemonNotInstalledResults = map[string]string{
	"stop":      "Background service is not installed; asked the service manager to stop any leftover job.",
	"uninstall": "Background service is not installed; asked the service manager to remove any leftover job.",
}

func applyDaemonNotInstalledPolicy(subcommand string, policy daemonNotInstalledPolicy) (notInstalled bool, code int, refused bool) {
	if policy == daemonProceed {
		return false, 0, false
	}

	installed, err := daemonInstallationStatus()
	if err != nil {
		if policy == daemonRefuse {
			fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.ErrorStyle,
				fmt.Sprintf("cannot determine whether the background service is installed; refusing to %s: %v", subcommand, err)))
			return false, 1, true
		}
		return false, 0, false
	}
	if installed {
		return false, 0, false
	}
	if policy == daemonRefuse {
		fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.ErrorStyle,
			"background service is not installed; install it first with: pinchtab daemon install"))
		return false, 1, true
	}
	return true, 0, false
}

func daemonLifecycleMessage(subcommand string, notInstalled bool, message string) string {
	if notInstalled {
		return daemonNotInstalledResults[subcommand]
	}
	return message
}

func printDaemonOK(message string) {
	fmt.Println(cli.StyleStdout(cli.SuccessStyle, "  [ok] ") + message)
}

func handleDaemonInstall(manager daemon.Manager) {
	configPath, fileCfg, _, err := daemon.EnsureConfig(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.ErrorStyle, fmt.Sprintf("daemon install failed: %v", err)))
		os.Exit(1)
	}
	if config.NeedsWizard(fileCfg) {
		isNew := config.IsFirstRun(fileCfg)
		runSecurityWizard(fileCfg, configPath, isNew)
	}
	if err := manager.Preflight(); err != nil {
		fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.ErrorStyle, fmt.Sprintf("daemon install unavailable: %v", err)))
		os.Exit(1)
	}
	message, err := manager.Install(configPath)
	if err != nil {
		printDaemonActionError(manager, fmt.Sprintf("daemon install failed: %v", err))
	}
	printDaemonOK(message)
	warnPrimaryChromeMacOS(loadConfig())
	printDaemonFollowUp()
}

func warnPrimaryChromeMacOS(cfg *config.RuntimeConfig) {
	effective := runtimekit.ResolveEffectiveBrowser(cfg)
	if effective.ID != config.BrowserChrome || !chrome.IsPrimaryChromeBinaryMacOS(effective.Binary) {
		return
	}
	fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.WarningStyle,
		"  [warn] Automation will use your primary Google Chrome on macOS."))
	fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.MutedStyle,
		"         Launching it headless can stop your normal Chrome from opening (issue #583)."))
	fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.MutedStyle,
		"         Install Google Chrome for Testing or Chromium, or set browser.binary in config"))
	fmt.Fprintln(os.Stderr, cli.StyleStderr(cli.MutedStyle,
		"         to a dedicated automation browser."))
}

func handleDaemonStop(manager daemon.Manager, notInstalled bool) {
	message, err := manager.Stop()
	if err != nil {
		printDaemonManagerResult(message, err)
		return
	}
	printDaemonOK(daemonLifecycleMessage("stop", notInstalled, message))
}

func handleDaemonUninstall(manager daemon.Manager, notInstalled bool) {
	message, err := manager.Uninstall()
	if err != nil {
		printDaemonActionError(manager, err.Error())
	}
	printDaemonOK(daemonLifecycleMessage("uninstall", notInstalled, message))
}
