//go:build !windows

package bridge

func isLockError(_ error) bool {
	return false
}
