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
				go outsiders.systemMetrics()
				go outsiders.applicationMetrics()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				outsiders.log.I("Stopping outsiders services")
				if err := outsiders.ss.Shutdown(ctx); err != nil {
					outsiders.log.F("Failed to shutdown startup server", tracing.OutsiderKind, "startup", tracing.InnerError, err)
				}
				if err := outsiders.sms.Shutdown(ctx); err != nil {
					outsiders.log.F("Failed to shutdown system metrics server", tracing.OutsiderKind, "system_metrics", tracing.InnerError, err)
				}
				if err := outsiders.as.Shutdown(ctx); err != nil {
					outsiders.log.F("Failed to shutdown application metrics server", tracing.OutsiderKind, "application_metrics", tracing.InnerError, err)
				}
				return nil
			},
		})
	}),
)