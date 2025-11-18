package personality

import (
	"go.uber.org/fx"
)

var Module = fx.Module("personality", fx.Provide(NewXiPersonality))
