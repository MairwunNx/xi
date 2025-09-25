package artificial

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	openrouter "github.com/revrost/go-openrouter"
)

// AgentResponse represents the response from context selection agent
type ContextSelectionResponse struct {
	RelevantIndices []int  `json:"relevant_indices"`
	Reasoning       string `json:"reasoning"`
}

// ModelSelectionResponse represents the response from model selection agent
type ModelSelectionResponse struct {
	RecommendedModel    string `json:"recommended_model"`
	ReasoningEffort     string `json:"reasoning_effort"`
	TaskComplexity      string `json:"task_complexity"`
	RequiresSpeed       bool   `json:"requires_speed"`
	RequiresQuality     bool   `json:"requires_quality"`
	IsTrolling          bool   `json:"is_trolling"`
	Reasoning           string `json:"reasoning"`
}

// AgentSystem handles the agent-based AI workflow
type AgentSystem struct {
	ai     *openrouter.Client
	config *AIConfig
}

func NewAgentSystem(ai *openrouter.Client, config *AIConfig) *AgentSystem {
	return &AgentSystem{
		ai:     ai,
		config: config,
	}
}

// SelectRelevantContext uses an agent to determine which messages from history are relevant
func (a *AgentSystem) SelectRelevantContext(
	log *tracing.Logger,
	history []platform.RedisMessage,
	newUserMessage string,
	userGrade platform.UserGrade,
) ([]platform.RedisMessage, error) {
	if len(history) == 0 {
		return []platform.RedisMessage{}, nil
	}

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
	defer cancel()

	// Create a lightweight prompt for context selection
	historyText := a.formatHistoryForAgent(history)
	
	prompt := a.getContextSelectionPrompt()
	
	systemMessage := fmt.Sprintf(prompt, historyText, newUserMessage)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Analyze the conversation history and select relevant messages for the new user question. Return your response in the specified JSON format."},
		},
	}

	// Use a fast, cheap model for context selection
	model := "openai/gpt-4o-mini"
	
	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1, // Low temperature for consistent analysis
	}

	log = log.With("ai_agent", "context_selector", tracing.AiModel, model)

	response, err := a.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		log.E("Failed to get context selection", tracing.InnerError, err)
		// Fallback: return last 5 messages
		return a.getLastNMessages(history, 5), nil
	}

	responseText := response.Choices[0].Message.Content.Text
	
	var contextResponse ContextSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &contextResponse); err != nil {
		log.E("Failed to parse context selection response", tracing.InnerError, err)
		// Fallback: return last 5 messages
		return a.getLastNMessages(history, 5), nil
	}

	// Extract relevant messages based on indices
	relevantMessages := []platform.RedisMessage{}
	for _, index := range contextResponse.RelevantIndices {
		if index >= 0 && index < len(history) {
			relevantMessages = append(relevantMessages, history[index])
		}
	}

	log.I("Context selection completed", 
		"original_count", len(history),
		"selected_count", len(relevantMessages),
		"reasoning", contextResponse.Reasoning,
	)

	return relevantMessages, nil
}

// SelectModelAndEffort uses an agent to determine the best model and reasoning effort
func (a *AgentSystem) SelectModelAndEffort(
	log *tracing.Logger,
	selectedContext []platform.RedisMessage,
	newUserMessage string,
	userGrade platform.UserGrade,
) (*ModelSelectionResponse, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	// Get tier policy for the user
	tierPolicy := a.getTierPolicy(userGrade)
	
	contextText := a.formatHistoryForAgent(selectedContext)
	
	prompt := a.getModelSelectionPrompt()
	
	systemMessage := fmt.Sprintf(prompt, 
		tierPolicy.ModelsText,
		tierPolicy.DefaultReasoning,
		tierPolicy.Description,
		contextText,
		newUserMessage,
	)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Analyze the task and recommend the optimal model and reasoning effort. Return your response in the specified JSON format."},
		},
	}

	// Use a fast, cheap model for model selection
	model := "openai/gpt-4o-mini"
	
	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.2,
	}

	log = log.With("ai_agent", "model_selector", tracing.AiModel, model)

	response, err := a.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		log.E("Failed to get model selection", tracing.InnerError, err)
		// Fallback to default tier settings
		gradeLimits := a.config.GradeLimits[userGrade]
		return &ModelSelectionResponse{
			RecommendedModel: gradeLimits.DialerPrimaryModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Reasoning:        "Fallback to default settings due to agent failure",
		}, nil
	}

	responseText := response.Choices[0].Message.Content.Text
	
	var modelResponse ModelSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &modelResponse); err != nil {
		log.E("Failed to parse model selection response", tracing.InnerError, err)
		// Fallback to default tier settings
		gradeLimits := a.config.GradeLimits[userGrade]
		return &ModelSelectionResponse{
			RecommendedModel: gradeLimits.DialerPrimaryModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Reasoning:        "Fallback to default settings due to parsing failure",
		}, nil
	}

	// Validate and adjust based on tier policy
	modelResponse = a.validateModelSelection(modelResponse, userGrade)

	log.I("Model selection completed",
		"recommended_model", modelResponse.RecommendedModel,
		"reasoning_effort", modelResponse.ReasoningEffort,
		"task_complexity", modelResponse.TaskComplexity,
		"is_trolling", modelResponse.IsTrolling,
		"reasoning", modelResponse.Reasoning,
	)

	return &modelResponse, nil
}

// Helper functions

func (a *AgentSystem) formatHistoryForAgent(history []platform.RedisMessage) string {
	var parts []string
	for i, msg := range history {
		role := "User"
		if msg.Role == platform.MessageRoleAssistant {
			role = "Assistant"
		}
		parts = append(parts, fmt.Sprintf("[%d] %s: %s", i, role, msg.Content))
	}
	return strings.Join(parts, "\n")
}

func (a *AgentSystem) getLastNMessages(history []platform.RedisMessage, n int) []platform.RedisMessage {
	if len(history) <= n {
		return history
	}
	return history[len(history)-n:]
}

type TierPolicy struct {
	ModelsText       string
	DefaultReasoning string
	Description      string
}

func (a *AgentSystem) getTierPolicy(userGrade platform.UserGrade) TierPolicy {
	switch userGrade {
	case platform.GradeGold:
		return TierPolicy{
			ModelsText: "anthropic/claude-opus-4.1, anthropic/claude-sonnet-4, google/gemini-2.5-pro, anthropic/claude-sonnet-3.7, openai/gpt-5, openai/gpt-4.1",
			DefaultReasoning: "high",
			Description: "Gold tier: максимальное качество, глубина размышлений, высокая надёжность",
		}
	case platform.GradeSilver:
		return TierPolicy{
			ModelsText: "google/gemini-2.5-pro, anthropic/claude-sonnet-3.7, x-ai/grok-3, openai/gpt-4.1, x-ai/grok-4",
			DefaultReasoning: "medium",
			Description: "Silver tier: баланс качества и стоимости, низкая задержка",
		}
	case platform.GradeBronze:
		return TierPolicy{
			ModelsText: "anthropic/claude-3.5-sonnet, openai/gpt-4.1, google/gemini-2.5-flash",
			DefaultReasoning: "medium",
			Description: "Bronze tier: минимальная стоимость и задержка",
		}
	default:
		return TierPolicy{
			ModelsText: "anthropic/claude-3.5-sonnet, openai/gpt-4.1",
			DefaultReasoning: "low",
			Description: "Default tier",
		}
	}
}

func (a *AgentSystem) validateModelSelection(response ModelSelectionResponse, userGrade platform.UserGrade) ModelSelectionResponse {
	tierPolicy := a.getTierPolicy(userGrade)
	availableModels := strings.Split(tierPolicy.ModelsText, ", ")
	
	// Check if recommended model is available for this tier
	modelValid := false
	for _, model := range availableModels {
		if model == response.RecommendedModel {
			modelValid = true
			break
		}
	}
	
	if !modelValid {
		// Fallback to first model in tier policy
		response.RecommendedModel = availableModels[0]
	}
	
	// Handle trolling case
	if response.IsTrolling {
		if len(a.config.TrollingModels) > 0 {
			response.RecommendedModel = a.config.TrollingModels[0]
		}
		response.ReasoningEffort = "low"
	}
	
	// Validate reasoning effort based on tier
	switch userGrade {
	case platform.GradeBronze:
		if response.ReasoningEffort == "high" {
			response.ReasoningEffort = "medium"
		}
	}
	
	return response
}

// Prompt getters - these are loaded from configuration with base64 decoding
func (a *AgentSystem) getContextSelectionPrompt() string {
	return a.config.GetContextSelectionPrompt()
}

func (a *AgentSystem) getModelSelectionPrompt() string {
	return a.config.GetModelSelectionPrompt()
}