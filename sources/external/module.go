package external

import (
	"context"
	"ximanager/sources/tracing"

	"go.uber.org/fx"
)

var Module = fx.Module("external", 
	fx.Provide(
		NewOutsidersConfig,
		NewOutsiders,
	),

	fx.Invoke(func(outsiders *Outsiders, lc fx.Lifecycle) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				outsiders.log.I("Starting outsiders services")
				go outsiders.startup()
				go outsiders.metrics()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				outsiders.log.I("Stopping outsiders services")
				if err := outsiders.ss.Shutdown(ctx); err != nil {
					outsiders.log.F("Failed to shutdown startup server", tracing.OutsiderKind, "startup", tracing.InnerError, err)
				}
				if err := outsiders.ms.Shutdown(ctx); err != nil {
					outsiders.log.F("Failed to shutdown metrics server", tracing.OutsiderKind, "metrics", tracing.InnerError, err)
				}
				return nil
			},
		})
	}),
) 