package collector

import "go.uber.org/fx"

var Module = fx.Module("metrics_collector",
	fx.Provide(
		NewStatsCollector,
	),
	fx.Invoke(func(*StatsCollector) {}),
)