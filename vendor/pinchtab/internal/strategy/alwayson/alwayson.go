package alwayson

import (
	"github.com/pinchtab/pinchtab/internal/strategy"
	"github.com/pinchtab/pinchtab/internal/strategy/autorestart"
)

func init() {
	// Defaults here are used if SetRuntimeConfig is not called.
	// In dashboard mode, SetRuntimeConfig overrides these from config file.
	strategy.MustRegister("always-on", func() strategy.Strategy {
		return autorestart.New(autorestart.AutorestartConfig{
			MaxRestarts:  -1,
			StrategyName: "always-on",
			StatusPath:   "/always-on/status",
		})
	})
}
