package artificial

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

type ModelMeta struct {
	Name            string `json:"name"`
	AAI             int    `json:"aai"`
	InputPricePerM  string `json:"input_price_per_m"`
	OutputPricePerM string `json:"output_price_per_m"`
	CtxTokens       string `json:"ctx_tokens"`
}

type GradeLimits struct {
	DialerModels          []ModelMeta
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

func (c *AIConfig) Validate() error {
	if err := platform.ValidateOpenAIToken(c.OpenAIToken); err != nil {
		return err
	}
	
	if err := platform.ValidateBase64(c.ContextSelectionPrompt, "ContextSelectionPrompt"); err != nil {
		return err
	}
	
	if err := platform.ValidateBase64(c.ModelSelectionPrompt, "ModelSelectionPrompt"); err != nil {
		return err
	}
	
	return nil
}

func parseModelsJSON(jsonStr string, defaultModels []ModelMeta) []ModelMeta {
	if jsonStr == "" {
		return defaultModels
	}
	
	var models []ModelMeta
	if err := json.Unmarshal([]byte(jsonStr), &models); err != nil {
		return defaultModels
	}
	
	return models
}

// Format: - `<model_name>` — AAI <0-100> | Input ($/1M tokens): <input price> | Output ($/1M tokens): <output price> | context tokens: <tokens>
func formatModelsForPrompt(models []ModelMeta) string {
	var lines []string
	for _, model := range models {
		line := fmt.Sprintf("- `%s` — AAI %d | Input ($/1M tokens): %s | Output ($/1M tokens): %s | context tokens: %s",
			model.Name,
			model.AAI,
			model.InputPricePerM,
			model.OutputPricePerM,
			model.CtxTokens,
		)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// extractModelNames extracts model names from ModelMeta slice
func extractModelNames(models []ModelMeta) []string {
	names := make([]string, len(models))
	for i, model := range models {
		names[i] = model.Name
	}
	return names
}

func NewAIConfig() *AIConfig {
	config := &AIConfig{
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
				DialerModels: parseModelsJSON(
					platform.Get("BRONZE_DIALER_MODELS", ""),
					[]ModelMeta{
						{Name: "anthropic/claude-3.5-sonnet", AAI: 75, InputPricePerM: "$3", OutputPricePerM: "$15", CtxTokens: "200k"},
						{Name: "openai/gpt-4.1", AAI: 80, InputPricePerM: "$2.5", OutputPricePerM: "$10", CtxTokens: "128k"},
						{Name: "google/gemini-2.5-flash", AAI: 70, InputPricePerM: "$0.075", OutputPricePerM: "$0.3", CtxTokens: "1M"},
					},
				),
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
				DialerModels: parseModelsJSON(
					platform.Get("SILVER_DIALER_MODELS", ""),
					[]ModelMeta{
						{Name: "google/gemini-2.5-pro", AAI: 85, InputPricePerM: "$1.25", OutputPricePerM: "$5", CtxTokens: "2M"},
						{Name: "anthropic/claude-3.7-sonnet", AAI: 87, InputPricePerM: "$3", OutputPricePerM: "$15", CtxTokens: "200k"},
						{Name: "x-ai/grok-3", AAI: 83, InputPricePerM: "$2", OutputPricePerM: "$10", CtxTokens: "128k"},
						{Name: "openai/gpt-4.1", AAI: 80, InputPricePerM: "$2.5", OutputPricePerM: "$10", CtxTokens: "128k"},
						{Name: "x-ai/grok-4", AAI: 88, InputPricePerM: "$5", OutputPricePerM: "$15", CtxTokens: "128k"},
					},
				),
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
				DialerModels: parseModelsJSON(
					platform.Get("GOLD_DIALER_MODELS", ""),
					[]ModelMeta{
						{Name: "anthropic/claude-sonnet-4.5", AAI: 95, InputPricePerM: "$15", OutputPricePerM: "$75", CtxTokens: "400k"},
						{Name: "anthropic/claude-sonnet-4", AAI: 93, InputPricePerM: "$12", OutputPricePerM: "$60", CtxTokens: "200k"},
						{Name: "openai/gpt-5", AAI: 92, InputPricePerM: "$10", OutputPricePerM: "$30", CtxTokens: "256k"},
						{Name: "google/gemini-2.5-pro", AAI: 85, InputPricePerM: "$1.25", OutputPricePerM: "$5", CtxTokens: "2M"},
						{Name: "anthropic/claude-3.7-sonnet", AAI: 87, InputPricePerM: "$3", OutputPricePerM: "$15", CtxTokens: "200k"},
						{Name: "openai/gpt-4.1", AAI: 80, InputPricePerM: "$2.5", OutputPricePerM: "$10", CtxTokens: "128k"},
					},
				),
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
	
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid AI configuration: %v", err))
	}
	
	return config
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
	return `You are a context selection agent. Your task is to analyze a conversation history and select which messages are relevant for answering the new user question.

Conversation history:
%s

New user question:
%s

Your goal: include all messages that help the model correctly understand and answer the new question.

Evaluate relevance using these principles (in order of importance):

1. **Direct reference** — If the new question explicitly refers to earlier content (e.g. “as before”, “that idea”, “continue”), include the full referenced part.
2. **Logical flow** — If several messages form a connected reasoning chain (e.g. question → clarification → response → follow-up), include the entire chain, not just the final message.
3. **Topical relation** — Include messages that are about the same or closely related topic, even if phrased differently.
4. **Recent context** — Prefer messages near the end of the conversation if they help understand tone, focus, or ongoing context.
5. **Continuity bias** — When uncertain, include *slightly more* context rather than too little.

If the new question is completely unrelated to previous topics, return an empty list.

💡 Tip: You can select **ranges** to keep the output compact when messages are consecutive.
Examples:
- Single messages: "5", "12"
- Ranges: "3-7" (includes 3,4,5,6,7)
- Mixed: ["0", "3-7", "12", "15-20"]

Return **only** JSON in this exact format:
{
  "relevant_indices": ["0", "3-7", "12"]
}`
}

func getDefaultModelSelectionPrompt() string {
	return `You are a model selection agent. Your job is to analyze a user task and recommend the most efficient AI model and reasoning effort — balancing quality, speed, and cost.

Available models for this tier (from MOST to LEAST capable), with AAI score (0–100), price per 1M tokens, and context window:

%s

Default reasoning effort for this tier: "%s"
Tier description: "%s"

%s

Core rules:
- Start from the **top model**, then **step down** to cheaper/faster ones if the task is simple, short, or routine.
- Stay within the user's tier whenever possible — they paid for its quality.
- Choose the **smallest capable model** that can reliably complete the task.
- Avoid using top-tier + high reasoning for trivial or conversational turns.
- If unsure, pick **medium reasoning**, not high.

When to downgrade:
- Task is short or factual (≤ 6 sentences)  
- Code edit is small/local  
- Simple math, rephrase, or obvious continuation  
- Light conversation or banter with Xi Manager  
- No high-stakes accuracy (e.g. legal/finance/medical)

When to stay high-tier:
- Multi-step reasoning, novel code, deep research  
- User requests "detailed", "in-depth", "thorough"  
- High-risk or high-importance tasks

Special cases:
- "Quick"/"fast" → prioritize speed + low reasoning  
- "Detailed"/"thorough" → prioritize quality + higher reasoning  
- Trolling/testing/nonsense → use trolling models (%s)

Heuristic:
Ask yourself: “Would an average competent model solve this correctly in one pass?”  
→ If yes, downgrade + low/medium reasoning.

Recent conversation context:
"""
%s
"""

New user task:
"""
%s
"""

Return only JSON in this format:
{
  "recommended_model": "exact model name from available list",
  "reasoning_effort": "low/medium/high",
  "task_complexity": "low/medium/high",
  "requires_speed": true/false,
  "requires_quality": true/false,
  "is_trolling": true/false
}`
}
