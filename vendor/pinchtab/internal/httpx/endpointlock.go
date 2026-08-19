package httpx

import (
	"fmt"
	"net/http"

	"github.com/pinchtab/pinchtab/internal/remedy"
)

// DisabledEndpointMessage returns a consistent message for locked endpoint
// families. capability names the gated capability, which is not always the
// endpoint the caller asked for — /storage is gated by stateExport — so the
// sentence names the requirement instead of calling the label an endpoint.
func DisabledEndpointMessage(capability, setting string) string {
	return fmt.Sprintf("this endpoint requires the %s capability; enable %s in config to use it", capability, setting)
}

// The remedy names the restart because writing the config is not enough: the
// security block is snapshotted at boot and is not rebuilt on a config edit, so
// enabling a capability never takes effect until the server restarts. Without
// the restart a caller who follows the remedy verbatim gets the identical 403
// back, and an agent — which reads this refusal, not the config command's
// stdout — has no exit from that loop.
var enableCapability = remedy.Declare("pinchtab config set <setting> true && pinchtab server restart")

// DisabledEndpointDetails builds the error details every capability refusal
// carries, so the guidance is written once rather than restated at each gate.
func DisabledEndpointDetails(setting string) map[string]any {
	details := remedy.Details(
		fmt.Sprintf("Enable %s to use this feature.", setting),
		enableCapability.Fill(setting))
	details["setting"] = setting
	return details
}

// DisabledEndpointHandler returns a handler that reports a capability gate lock.
func DisabledEndpointHandler(capability, setting, code string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ErrorCode(w, http.StatusForbidden, code,
			DisabledEndpointMessage(capability, setting), false,
			DisabledEndpointDetails(setting))
	}
}
