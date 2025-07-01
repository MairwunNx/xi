package artificial

import (
	"ximanager/sources/platform"

	"github.com/sashabaranov/go-openai"
)

type AIConfig struct {
	OpenRouterToken string
	OpenAIToken string

	WhisperModel string

	DialerPrimaryModel string
	DialerFallbackModels []string
	DialerReasoningEffort string

	VisionPrimaryModel string
	VisionFallbackModels []string
}

func NewAIConfig() *AIConfig {
	return &AIConfig{
		OpenRouterToken: platform.Get("OPENROUTER_API_KEY", ""),
		OpenAIToken: platform.Get("OPENAI_API_KEY", ""),

		WhisperModel: platform.Get("WHISPER_MODEL", openai.Whisper1),

		DialerPrimaryModel: platform.Get("DIALER_PRIMARY_MODEL", "openai/gpt-4.1"),
		DialerFallbackModels: platform.GetAsSlice("DIALER_FALLBACK_MODELS", []string{"google/gemini-2.5-pro", "anthropic/claude-sonnet-4", "anthropic/claude-3.7-sonnet", "grok-beta", "deepseek/deepseek-r1"}),
		DialerReasoningEffort: platform.Get("DIALER_REASONING_EFFORT", "medium"),

		VisionPrimaryModel: platform.Get("VISION_PRIMARY_MODEL", "openai/gpt-4o"),
		VisionFallbackModels: platform.GetAsSlice("VISION_FALLBACK_MODELS", []string{"google/gemini-2.5-pro", "anthropic/claude-3.7-sonnet", "google/gemini-pro-vision"}),
	}
}