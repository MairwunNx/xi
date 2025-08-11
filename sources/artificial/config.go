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
	GradeLimits    map[platform.UserGrade]GradeLimits
}

type GradeLimits struct {
	Context ContextLimits
	Usage   UsageLimits
}

type ContextLimits struct {
	TTL         int
	MaxMessages int
	MaxTokens   int
}

type UsageLimits struct {
	VisionDaily   int
	VisionMonthly int
	DialerDaily   int
	DialerMonthly int
	WhisperDaily  int
	WhisperMonthly int
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

		DialerPrimaryModel: platform.Get("DIALER_PRIMARY_MODEL", "openai/o3-pro"),
		DialerFallbackModels: platform.GetAsSlice("DIALER_FALLBACK_MODELS", []string{"google/gemini-2.5-pro", "anthropic/claude-sonnet-4", "x-ai/grok-3"}),
		DialerReasoningEffort: platform.Get("DIALER_REASONING_EFFORT", "medium"),

		VisionPrimaryModel: platform.Get("VISION_PRIMARY_MODEL", "openai/o3-pro"),
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
		GradeLimits: map[platform.UserGrade]GradeLimits{
			platform.GradeBronze: {
				Context: ContextLimits{
					TTL:         platform.GetAsInt("BRONZE_CONTEXT_TTL_SECONDS", 1800),
					MaxMessages: platform.GetAsInt("BRONZE_CONTEXT_MAX_MESSAGES", 25),
					MaxTokens:   platform.GetAsInt("BRONZE_CONTEXT_MAX_TOKENS", 40000),
				},
				Usage: UsageLimits{
					VisionDaily:   platform.GetAsInt("BRONZE_USAGE_VISION_DAILY", 3),
					VisionMonthly: platform.GetAsInt("BRONZE_USAGE_VISION_MONTHLY", 20),
					DialerDaily:   platform.GetAsInt("BRONZE_USAGE_DIALER_DAILY", 40),
					DialerMonthly: platform.GetAsInt("BRONZE_USAGE_DIALER_MONTHLY", 300),
					WhisperDaily:  platform.GetAsInt("BRONZE_USAGE_WHISPER_DAILY", 15),
					WhisperMonthly: platform.GetAsInt("BRONZE_USAGE_WHISPER_MONTHLY", 70),
				},
			},
			platform.GradeSilver: {
				Context: ContextLimits{
					TTL:         platform.GetAsInt("SILVER_CONTEXT_TTL_SECONDS", 7200),
					MaxMessages: platform.GetAsInt("SILVER_CONTEXT_MAX_MESSAGES", 40),
					MaxTokens:   platform.GetAsInt("SILVER_CONTEXT_MAX_TOKENS", 70000),
				},
				Usage: UsageLimits{
					VisionDaily:   platform.GetAsInt("SILVER_USAGE_VISION_DAILY", 7),
					VisionMonthly: platform.GetAsInt("SILVER_USAGE_VISION_MONTHLY", 35),
					DialerDaily:   platform.GetAsInt("SILVER_USAGE_DIALER_DAILY", 150),
					DialerMonthly: platform.GetAsInt("SILVER_USAGE_DIALER_MONTHLY", 600),
					WhisperDaily:  platform.GetAsInt("SILVER_USAGE_WHISPER_DAILY", 30),
					WhisperMonthly: platform.GetAsInt("SILVER_USAGE_WHISPER_MONTHLY", 170),
				},
			},
			platform.GradeGold: {
				Context: ContextLimits{
					TTL:         platform.GetAsInt("GOLD_CONTEXT_TTL_SECONDS", 21600),
					MaxMessages: platform.GetAsInt("GOLD_CONTEXT_MAX_MESSAGES", 60),
					MaxTokens:   platform.GetAsInt("GOLD_CONTEXT_MAX_TOKENS", 160000),
				},
				Usage: UsageLimits{
					VisionDaily:   platform.GetAsInt("GOLD_USAGE_VISION_DAILY", 20),
					VisionMonthly: platform.GetAsInt("GOLD_USAGE_VISION_MONTHLY", 100),
					DialerDaily:   platform.GetAsInt("GOLD_USAGE_DIALER_DAILY", 300),
					DialerMonthly: platform.GetAsInt("GOLD_USAGE_DIALER_MONTHLY", 1500),
					WhisperDaily:  platform.GetAsInt("GOLD_USAGE_WHISPER_DAILY", 140),
					WhisperMonthly: platform.GetAsInt("GOLD_USAGE_WHISPER_MONTHLY", 300),
				},
			},
		},
	}
}