package artificial

import "go.uber.org/fx"

var Module = fx.Module("artificial",
	fx.Provide(
		NewAIConfig,
		NewOrchestratorConfig,
		NewOpenAIClient,
		NewDeepseekClient,
		NewGrokClient,
		NewAnthropicClient,
		NewAIClientsMap,
		NewTopicAnalyzer,
		NewOrchestrator,
	),
)
