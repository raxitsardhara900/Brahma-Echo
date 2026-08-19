package strategy

import (
	"net/http"

	"github.com/pinchtab/pinchtab/internal/orchestrator"
	"github.com/pinchtab/pinchtab/internal/routes"
)

func capabilitySetting(cap routes.Capability) (feature, setting, code string) {
	if meta, ok := routes.Meta(cap); ok {
		return meta.Label, meta.Setting, meta.DisabledCode
	}
	return string(cap), "", ""
}

func RegisterShorthandRoutes(mux *http.ServeMux, orch *orchestrator.Orchestrator, handler http.HandlerFunc) {
	wrapped := orch.WrapShorthand(handler)

	for _, route := range routes.ShorthandRoutes() {
		mux.HandleFunc(route, wrapped)
	}

	for cap, eps := range routes.CapabilityEndpoints() {
		enabled := orch.Allows(cap)
		feature, setting, code := capabilitySetting(cap)
		for _, ep := range eps {
			RegisterCapabilityRoute(mux, ep.Route(), enabled, feature, setting, code, wrapped)
		}
	}
}
