package texting

import (
	"go.uber.org/fx"
)

var Module = fx.Module("texting",
	fx.Provide(
		NewXiMessenger,
	),
)