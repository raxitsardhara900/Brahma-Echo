package bridge

import "fmt"

// TabLimitError is returned when a new tab cannot be created because
// the configured limit has been reached and the eviction policy is "reject".
// HTTP handlers should map this to 429 Too Many Requests.
type TabLimitError struct {
	Current int
	Max     int
}

func (e *TabLimitError) Error() string {
	return fmt.Sprintf("tab limit reached (%d/%d)", e.Current, e.Max)
}

// ErrDialogBlocking is returned when a click action is blocked by a
// JavaScript dialog (alert/confirm/prompt) and no --dialog-action was provided.
type ErrDialogBlocking struct {
	DialogType    string
	DialogMessage string
}

func (e *ErrDialogBlocking) Error() string {
	return fmt.Sprintf("click blocked by JavaScript dialog (%s: %q) — use --dialog-action accept|dismiss", e.DialogType, e.DialogMessage)
}
