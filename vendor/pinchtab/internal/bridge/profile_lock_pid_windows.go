//go:build windows

package bridge

func isChromePIDRunning(pid int) (bool, error) {
	_ = pid
	return false, nil
}

func killProcesses(processes []chromeProfileProcess) error {
	_ = processes
	return nil
}

func isPinchTabProcess(pid int) bool {
	_ = pid
	return true // Better to assume true on Windows for now to avoid accidental deletions
}
