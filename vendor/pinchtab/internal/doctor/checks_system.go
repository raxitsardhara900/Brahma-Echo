package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/pinchtab/pinchtab/internal/config"
)

func checkConfigFile(_ context.Context, _ *config.RuntimeConfig) CheckResult {
	status := config.InspectConfigFile()
	if !status.Found {
		detail := fmt.Sprintf("not found at %s", status.Path)
		if status.EnvOverride {
			detail += " (PINCHTAB_CONFIG override; default would be " + status.DefaultPath + ")"
		} else {
			detail += " (default search path; set PINCHTAB_CONFIG to override)"
		}
		return CheckResult{Status: StatusWarn, Detail: detail}
	}
	if status.ParseErr != nil {
		return CheckResult{
			Status: StatusFail,
			Detail: fmt.Sprintf("%s: parse error: %v", status.Path, status.ParseErr),
			Err:    status.ParseErr,
		}
	}
	// A typo in a config key loads fine and yields a working server with silently
	// wrong settings, so "loaded" alone is not a clean bill of health: name the
	// keys that were ignored.
	if len(status.UnknownKeys) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: fmt.Sprintf("%s (loaded) — unrecognized keys ignored: %s", status.Path, strings.Join(status.UnknownKeys, ", ")),
		}
	}
	return CheckResult{
		Status: StatusPass,
		Detail: status.Path + " (loaded)",
	}
}
