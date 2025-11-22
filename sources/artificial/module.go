package artificial

import "go.uber.org/fx"

var Module = fx.Module(
	"artificial",
	fx.Provide(
		NewOpenRouterClient,
		NewOpenAIClient,
		NewContextManager,
		NewUsageLimiter,
		NewSpendingLimiter,
		NewDialer,
		NewWhisper,
		NewAgentSystem,
	),
)