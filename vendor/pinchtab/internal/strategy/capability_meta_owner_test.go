package strategy

import (
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/routes"
)

// The same route answering two different disabled codes depending on which front a client
// reached is what a second copy of this table produced. Both fronts gate from
// CapabilityEndpoints, so every capability it yields must resolve here to exactly what
// routes.Meta says — no restated labels, no synthesised settings.
func TestEveryGatedCapabilityResolvesToTheOneOwner(t *testing.T) {
	gated := routes.CapabilityEndpoints()
	if len(gated) == 0 {
		t.Fatal("no capability-gated endpoints found; this census would pass vacuously")
	}

	for cap := range gated {
		meta, ok := routes.Meta(cap)
		if !ok {
			t.Errorf("capability %q gates routes but routes.Meta does not describe it, so this front has nothing to answer with", cap)
			continue
		}

		feature, setting, code := capabilitySetting(cap)
		if feature != meta.Label || setting != meta.Setting || code != meta.DisabledCode {
			t.Errorf("capability %q resolves to (%q, %q, %q) but routes.Meta says (%q, %q, %q); a front that disagrees makes one capability answer two ways",
				cap, feature, setting, code, meta.Label, meta.Setting, meta.DisabledCode)
		}
	}
}

// The old fallback synthesised "security.allow"+cap and cap+"_disabled". That is wrong for
// any camelCase capability — which is why every one of them carried an explicit case — so a
// capability added to the catalogue and NOT to routes.Meta must fail loudly rather than be
// gated with a plausible-looking setting no config file has.
func TestAnUndescribedCapabilityDoesNotGetASynthesisedSetting(t *testing.T) {
	feature, setting, code := capabilitySetting(routes.Capability("stateExport-but-undescribed"))

	if setting != "" || code != "" {
		t.Errorf("an undescribed capability produced setting %q and code %q; a synthesised value reads as configuration that exists", setting, code)
	}
	if feature == "" {
		t.Error("the label should still name the capability, so a refusal is not anonymous")
	}
	if strings.Contains(setting, "security.allow") {
		t.Errorf("setting %q looks like a real config path but was invented here", setting)
	}
}
