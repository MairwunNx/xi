package localization

import (
	"go.uber.org/fx"
)

var Module = fx.Module("localization",
	fx.Provide(
		NewLanguageDetector,
		NewLocalizationManager,
	),
)