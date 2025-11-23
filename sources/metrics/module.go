package metrics

import "go.uber.org/fx"

var Module = fx.Module("metrics",
	fx.Provide(
		NewMetricsService,
	),
)