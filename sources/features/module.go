package features

import (
	"context"
	"ximanager/sources/tracing"

	"go.uber.org/fx"
)

var Module = fx.Module("features",
	fx.Provide(
		NewFeatureConfig,
		NewFeatureManager,
	),
	fx.Invoke(func(lc fx.Lifecycle, fm *FeatureManager, log *tracing.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				log.I("Feature toggles initialized")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return fm.OnStop(ctx)
			},
		})
	}),
)
