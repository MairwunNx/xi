package artificial

import "go.uber.org/fx"

var Module = fx.Module("artificial", fx.Provide(NewAIConfig, NewOpenRouterClient, NewOpenAIClient, NewSpendingLimiter, NewDialer, NewWhisper, NewVision))
