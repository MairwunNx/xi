package artificial

import (
	"ximanager/sources/platform"

	"github.com/sashabaranov/go-openai"
	"github.com/shopspring/decimal"
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

	LimitExceededModel string
	LimitExceededFallbackModels []string
	SpendingLimits SpendingLimits
}

type SpendingLimits struct {
	BronzeDailyLimit   decimal.Decimal
	BronzeMonthlyLimit decimal.Decimal
	SilverDailyLimit   decimal.Decimal
	SilverMonthlyLimit decimal.Decimal
	GoldDailyLimit     decimal.Decimal
	GoldMonthlyLimit   decimal.Decimal
}

func NewAIConfig() *AIConfig {
	return &AIConfig{
		OpenRouterToken: platform.Get("OPENROUTER_API_KEY", ""),
		OpenAIToken: platform.Get("OPENAI_API_KEY", ""),

		WhisperModel: platform.Get("WHISPER_MODEL", openai.Whisper1),

		DialerPrimaryModel: platform.Get("DIALER_PRIMARY_MODEL", "openai/gpt-4.1"),
		DialerFallbackModels: platform.GetAsSlice("DIALER_FALLBACK_MODELS", []string{"google/gemini-2.5-pro", "anthropic/claude-sonnet-4", "x-ai/grok-3"}),
		DialerReasoningEffort: platform.Get("DIALER_REASONING_EFFORT", "medium"),

		VisionPrimaryModel: platform.Get("VISION_PRIMARY_MODEL", "openai/gpt-4o"),
		VisionFallbackModels: platform.GetAsSlice("VISION_FALLBACK_MODELS", []string{"google/gemini-2.5-pro", "anthropic/claude-3.7-sonnet"}),

		LimitExceededModel: platform.Get("SPENDINGS_LIMIT_EXCEEDED_MODEL", "openai/gpt-4o-mini"),
		LimitExceededFallbackModels: platform.GetAsSlice("SPENDINGS_LIMIT_EXCEEDED_FALLBACK_MODELS", []string{"deepseek/deepseek-chat"}),

		SpendingLimits: SpendingLimits{
			BronzeDailyLimit:   platform.GetDecimal("SPENDINGS_BRONZE_DAILY_LIMIT", "2.0"),
			BronzeMonthlyLimit: platform.GetDecimal("SPENDINGS_BRONZE_MONTHLY_LIMIT", "7.0"),
			SilverDailyLimit:   platform.GetDecimal("SPENDINGS_SILVER_DAILY_LIMIT", "3.7"),
			SilverMonthlyLimit: platform.GetDecimal("SPENDINGS_SILVER_MONTHLY_LIMIT", "15.0"),
			GoldDailyLimit:     platform.GetDecimal("SPENDINGS_GOLD_DAILY_LIMIT", "5.0"),
			GoldMonthlyLimit:   platform.GetDecimal("SPENDINGS_GOLD_MONTHLY_LIMIT", "26.0"),
		},
	}
}