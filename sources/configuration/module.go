package configuration

import "go.uber.org/fx"

var Module = fx.Module("configuration",
	fx.Provide(
		NewYaml,
	),
)