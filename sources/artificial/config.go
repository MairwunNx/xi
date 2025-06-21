package artificial

import (
	"ximanager/sources/platform"

	"time"

	deepseek "github.com/cohesion-org/deepseek-go"
	"github.com/sashabaranov/go-openai"
)

type AIConfig struct {
	OpenAIToken string
	DeepseekToken string
	GrokToken string
	AnthropicToken string

	OpenAIModel string
	OpenAIImageModel string
	OpenAIAudioModel string
	OpenAILightweightModel string
	OpenAIMediumWeightModel string
	DeepseekModel string
	GrokModel string
	AnthropicModel string

	OpenAIMaxTokens int
	OpenAIImageMaxTokens int
	OpenAILightweightMaxTokens int
	OpenAIMediumWeightMaxTokens int
	DeepseekMaxTokens int
	GrokMaxTokens int
	AnthropicMaxTokens int
}

type OrchestratorConfig struct {
	MaxRetries int
	BackoffDelay time.Duration
}

func NewAIConfig() *AIConfig {
	return &AIConfig{
		OpenAIToken: platform.Get("OPENAI_API_KEY", ""),
		DeepseekToken: platform.Get("DEEPSEEK_API_KEY", ""),
		GrokToken: platform.Get("GROK_API_KEY", ""),
		AnthropicToken: platform.Get("ANTHROPIC_API_KEY", ""),

		OpenAIModel: platform.Get("OPENAI_MODEL", openai.GPT4Dot1),
		OpenAIImageModel: platform.Get("OPENAI_IMAGE_MODEL", openai.GPT4Dot1),
		OpenAIAudioModel: platform.Get("OPENAI_AUDIO_MODEL", openai.Whisper1),
		OpenAILightweightModel: platform.Get("OPENAI_LIGHTWEIGHT_MODEL", openai.GPT3Dot5Turbo),
		OpenAIMediumWeightModel: platform.Get("OPENAI_MEDIUMWEIGHT_MODEL", openai.GPT4oMini),
		DeepseekModel: platform.Get("DEEPSEEK_MODEL", deepseek.DeepSeekReasoner),
		GrokModel: platform.Get("GROK_MODEL", "grok-2-latest"),
		AnthropicModel: platform.Get("ANTHROPIC_MODEL", "claude-3-7-sonnet-latest"),

		OpenAIMaxTokens: platform.GetAsInt("OPENAI_MAX_TOKENS", 32768),
		OpenAIImageMaxTokens: platform.GetAsInt("OPENAI_IMAGE_MAX_TOKENS", 32768),
		OpenAILightweightMaxTokens: platform.GetAsInt("OPENAI_LIGHTWEIGHT_MAX_TOKENS", 4096),
		OpenAIMediumWeightMaxTokens: platform.GetAsInt("OPENAI_MEDIUMWEIGHT_MAX_TOKENS", 16384),
		DeepseekMaxTokens: platform.GetAsInt("DEEPSEEK_MAX_TOKENS", 12000),
		GrokMaxTokens: platform.GetAsInt("GROK_MAX_TOKENS", 12000),
		AnthropicMaxTokens: platform.GetAsInt("ANTHROPIC_MAX_TOKENS", 8000),
	}
}

func NewOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		MaxRetries: platform.GetAsInt("ORCHESTRATOR_MAX_RETRIES", 3),
		BackoffDelay: platform.GetAsDuration("ORCHESTRATOR_BACKOFF_DELAY", "500ms"),
	}
}