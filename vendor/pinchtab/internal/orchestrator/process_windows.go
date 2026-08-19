//go:build windows

package orchestrator

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

func setProcGroup(cmd *exec.Cmd) {}

func killProcessGroup(pid int, _ syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

var (
	modkernel32            = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess        = modkernel32.NewProc("OpenProcess")
	procGetExitCodeProcess = modkernel32.NewProc("GetExitCodeProcess")
)

const (
	processQueryLimitedInfo = 0x1000
	stillActive             = 259 // STATUS_PENDING — process has not exited
)

func processAlive(pid int) bool {
	// On Windows, os.Process.Signal(0) always fails because Go does not
	// support signals on Windows (only os.Kill via TerminateProcess).
	// Instead, use OpenProcess + GetExitCodeProcess to check liveness.
	handle, _, err := procOpenProcess.Call(
		uintptr(processQueryLimitedInfo),
		0, // bInheritHandle = false
		uintptr(pid),
	)
	if handle == 0 {
		// Cannot open process — either it doesn't exist or access denied.
		// If access denied the process is alive, but we can't tell; assume dead
		// to match the previous (broken) behaviour conservatively.
		_ = err
		return false
	}
	defer func() { _ = syscall.CloseHandle(syscall.Handle(handle)) }()

	var exitCode uint32
	ret, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false // API call failed
	}
	return exitCode == stillActive
}

const sigTERM = syscall.Signal(0xf)

const sigKILL = syscall.Signal(0x9)
