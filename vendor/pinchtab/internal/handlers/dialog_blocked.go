package handlers

import (
	"fmt"
	"net/http"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/httpx"
	"github.com/pinchtab/pinchtab/internal/remedy"
)

const dialogBlockedCode = "dialog_blocked"

const dialogBlockedHint = "a JavaScript dialog is open on this tab and blocks every page interaction until it is answered. It belongs to this tab, so retrying, activating the tab or opening a fresh one cannot clear it. Use pinchtab dialog dismiss to cancel it instead of accepting, or pass --dialog-action accept|dismiss on the action that opens the dialog."

// Accepting is the remedy because it is the answer that lets the blocked interaction
// proceed; dismissing is the other choice a reader may want and lives in the hint. The
// old value offered both as "accept|dismiss", which a shell reads as a pipeline.
var dialogBlockedRemedy = remedy.Declare("pinchtab dialog accept")

func pendingTabDialog(b bridge.BridgeAPI, tabID string) *bridge.DialogState {
	if b == nil || tabID == "" {
		return nil
	}
	dm := b.GetDialogManager()
	if dm == nil {
		return nil
	}
	return dm.GetPending(tabID)
}

func dialogBlockedMessage(tabID string, dialog *bridge.DialogState) string {
	return fmt.Sprintf("tab %s is blocked by a JavaScript dialog (%s: %q)", tabID, dialog.Type, dialog.Message)
}

func dialogBlockedDetails(tabID string, dialog *bridge.DialogState) map[string]any {
	details := remedy.Details(dialogBlockedHint, dialogBlockedRemedy.Remedy())
	details["tabId"] = tabID
	details["dialogType"] = dialog.Type
	details["dialogMessage"] = dialog.Message
	return details
}

func writeDialogBlocked(w http.ResponseWriter, tabID string, dialog *bridge.DialogState, message string) {
	if message == "" {
		message = dialogBlockedMessage(tabID, dialog)
	}
	httpx.ErrorCode(w, http.StatusConflict, dialogBlockedCode, message, false, dialogBlockedDetails(tabID, dialog))
}

// refuseIfDialogBlocked answers the request with dialog_blocked when a dialog is
// pending on the tab. The lookup is in-memory, so it never touches the page it is
// reporting on.
func (h *Handlers) refuseIfDialogBlocked(w http.ResponseWriter, tabID string) bool {
	dialog := pendingTabDialog(h.Bridge, tabID)
	if dialog == nil {
		return false
	}
	writeDialogBlocked(w, tabID, dialog, "")
	return true
}
