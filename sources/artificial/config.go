package artificial

import (
	"encoding/base64"
	"ximanager/sources/platform"

	"github.com/sashabaranov/go-openai"
	"github.com/shopspring/decimal"
)

type AIConfig struct {
	OpenRouterToken string
	OpenAIToken     string

	WhisperModel string

	LimitExceededModel          string
	LimitExceededFallbackModels []string
	SpendingLimits              SpendingLimits
	GradeLimits                 map[platform.UserGrade]GradeLimits
	
	ContextSelectionPrompt string
	ModelSelectionPrompt   string
	
	AgentContextModel        string
	AgentModelSelectionModel string
	AgentContextTimeout      int
	AgentModelTimeout        int
	
	TrollingModels []string
}

type GradeLimits struct {
	DialerModels          []string
	DialerReasoningEffort string
	VisionPrimaryModel    string
	VisionFallbackModels  []string
	Context               ContextLimits
	Usage                 UsageLimits
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
		OpenAIToken:     platform.Get("OPENAI_API_KEY", ""),

		WhisperModel: platform.Get("WHISPER_MODEL", openai.Whisper1),

		LimitExceededModel:          platform.Get("SPENDINGS_LIMIT_EXCEEDED_MODEL", "openai/gpt-4o-mini"),
		LimitExceededFallbackModels: platform.GetAsSlice("SPENDINGS_LIMIT_EXCEEDED_FALLBACK_MODELS", []string{"deepseek/deepseek-chat"}),
		
		ContextSelectionPrompt: platform.Get("AGENT_CONTEXT_SELECTION_PROMPT", ""),
		ModelSelectionPrompt:   platform.Get("AGENT_MODEL_SELECTION_PROMPT", ""),
		
		AgentContextModel:        platform.Get("AGENT_CONTEXT_MODEL", "openai/gpt-4o-mini"),
		AgentModelSelectionModel: platform.Get("AGENT_MODEL_SELECTION_MODEL", "openai/gpt-4o-mini"),
		AgentContextTimeout:      platform.GetAsInt("AGENT_CONTEXT_TIMEOUT_SECONDS", 45),
		AgentModelTimeout:        platform.GetAsInt("AGENT_MODEL_TIMEOUT_SECONDS", 30),
		
		TrollingModels: platform.GetAsSlice("TROLLING_MODELS", []string{"openai/gpt-4.1-mini", "x-ai/grok-4-fast", "x-ai/grok-4-fast:free"}),

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
				DialerModels:          platform.GetAsSlice("BRONZE_DIALER_MODELS", []string{"anthropic/claude-3.5-sonnet", "openai/gpt-4.1", "google/gemini-2.5-flash"}),
				DialerReasoningEffort: platform.Get("BRONZE_DIALER_REASONING_EFFORT", "medium"),
				VisionPrimaryModel:    platform.Get("BRONZE_VISION_PRIMARY_MODEL", "openai/chatgpt-4o-latest"),
				VisionFallbackModels:  platform.GetAsSlice("BRONZE_VISION_FALLBACK_MODELS", []string{}),
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
				DialerModels:          platform.GetAsSlice("SILVER_DIALER_MODELS", []string{"google/gemini-2.5-pro", "anthropic/claude-sonnet-3.7", "x-ai/grok-3", "openai/gpt-4.1", "x-ai/grok-4"}),
				DialerReasoningEffort: platform.Get("SILVER_DIALER_REASONING_EFFORT", "medium"),
				VisionPrimaryModel:    platform.Get("SILVER_VISION_PRIMARY_MODEL", "openai/gpt-4.1"),
				VisionFallbackModels:  platform.GetAsSlice("SILVER_VISION_FALLBACK_MODELS", []string{}),
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
				DialerModels:          platform.GetAsSlice("GOLD_DIALER_MODELS", []string{"anthropic/claude-opus-4.1", "anthropic/claude-sonnet-4", "openai/gpt-5", "google/gemini-2.5-pro", "anthropic/claude-sonnet-3.7", "openai/gpt-4.1"}),
				DialerReasoningEffort: platform.Get("GOLD_DIALER_REASONING_EFFORT", "high"),
				VisionPrimaryModel:    platform.Get("GOLD_VISION_PRIMARY_MODEL", "openai/o1"),
				VisionFallbackModels:  platform.GetAsSlice("GOLD_VISION_FALLBACK_MODELS", []string{}),
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

// GetContextSelectionPrompt returns the decoded context selection prompt
func (c *AIConfig) GetContextSelectionPrompt() string {
	decoded, err := base64.StdEncoding.DecodeString(c.ContextSelectionPrompt)
	if err != nil {
		return getDefaultContextSelectionPrompt()
	}
	
	return string(decoded)
}

// GetModelSelectionPrompt returns the decoded model selection prompt
func (c *AIConfig) GetModelSelectionPrompt() string {
	decoded, err := base64.StdEncoding.DecodeString(c.ModelSelectionPrompt)
	if err != nil {
		return getDefaultModelSelectionPrompt()
	}
	
	return string(decoded)
}

func getDefaultContextSelectionPrompt() string {
	return `You are a context selection agent. Your job is to analyze conversation history and determine which messages are relevant for answering a new user question.

Conversation history:
%s

New user question:
%s

Analyze the conversation and determine which messages from the history are relevant to answering the new question. Consider:
1. Topical relevance - are previous messages about the same or related topic?
2. Context dependency - does the new question reference or build upon previous messages?
3. Conversation flow - are there important context clues in recent messages?
4. User intent - if the user explicitly asks to "remember" or "continue" something, include relevant history.

If the new question is about a completely different topic and doesn't reference previous conversation, you may return an empty array.

Return your response as JSON in this exact format:
{
  "relevant_indices": [array of message indices that are relevant, e.g., [0, 2, 5]],
  "reasoning": "Brief explanation of why these messages were selected"
}`
}

func getDefaultModelSelectionPrompt() string {
	return `You are a model selection agent. Your job is to analyze a task and recommend the optimal AI model and reasoning effort.

Available models for this tier (from most capable to fastest/cheapest): %s
Default reasoning effort for this tier: %s
Tier description: %s

Recent conversation context:
%s

New user task:
%s

Analyze the task and recommend:
1. Which model to use (must be from the available models list)
2. What reasoning effort level (low/medium/high)
3. Task complexity assessment
4. Whether user needs speed vs quality
5. Whether this appears to be trolling/testing behavior

Consider:
- Complex coding, analysis, research tasks → higher capability model + higher reasoning
- Simple questions, quick answers → faster model + lower reasoning
- User requests for "quick" or "fast" → prioritize speed
- User requests for "detailed" or "thorough" → prioritize quality
- Nonsensical, repetitive, or obviously testing queries → use trolling models (%s)

Return your response as JSON in this exact format:
{
  "recommended_model": "exact model name from available list",
  "reasoning_effort": "low/medium/high",
  "task_complexity": "low/medium/high",
  "requires_speed": true/false,
  "requires_quality": true/false,
  "is_trolling": true/false,
  "reasoning": "Brief explanation of your choice"
}`
}
