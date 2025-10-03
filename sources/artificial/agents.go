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

// ContextSelectionResponse represents the response from context selection agent
type ContextSelectionResponse struct {
	RelevantIndices []string `json:"relevant_indices"`
	Reasoning       string   `json:"reasoning"`
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
	if len(history) <= 4 {
		return history, nil
	}

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AgentContextTimeout)*time.Second)
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

	model := a.config.AgentContextModel
	
	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
	}

	log = log.With("ai_agent", "context_selector", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)
	
	if err != nil {
		log.E("Failed to get context selection", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_context_selection_failed", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return history, nil
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in context selection response")
		log.I("agent_context_selection_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return history, nil
	}

	responseText := response.Choices[0].Message.Content.Text
	
	var contextResponse ContextSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &contextResponse); err != nil {
		log.E("Failed to parse context selection response", tracing.InnerError, err, "response_text", responseText)
		log.I("agent_context_selection_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return history, nil
	}

	// Parse indices and ranges into flat list of indices
	indices, err := a.parseIndicesAndRanges(contextResponse.RelevantIndices, len(history)-1)
	if err != nil {
		log.E("Failed to parse indices and ranges", tracing.InnerError, err)
		log.I("agent_context_selection_failed", "reason", "parse_indices_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return history, nil
	}

	// Extract relevant messages based on parsed indices
	relevantMessages := []platform.RedisMessage{}
	for _, index := range indices {
		if index >= 0 && index < len(history) {
			relevantMessages = append(relevantMessages, history[index])
		}
	}

	reductionPercent := 0
	if len(history) > 0 {
		reductionPercent = int(float64(len(history)-len(relevantMessages)) / float64(len(history)) * 100)
	}

	log.I("agent_context_selection_success", 
		"original_count", len(history),
		"selected_count", len(relevantMessages),
		"reduction_percent", reductionPercent,
		"raw_indices_and_ranges", contextResponse.RelevantIndices,
		"parsed_indices_count", len(indices),
		"agent_reasoning", contextResponse.Reasoning,
		"duration_ms", duration.Milliseconds(),
		"user_grade", userGrade,
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
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AgentModelTimeout)*time.Second)
	defer cancel()

	// Get tier policy for the user
	tierPolicy := a.getTierPolicy(userGrade)
	
	// Limit context to last 6 messages (3 pairs) to save tokens
	limitedContext := selectedContext
	if len(selectedContext) > 6 {
		limitedContext = selectedContext[len(selectedContext)-6:]
	}
	contextText := a.formatHistoryForAgent(limitedContext)
	
	prompt := a.getModelSelectionPrompt()
	
	trollingModelsText := strings.Join(a.config.TrollingModels, ", ")
	
	systemMessage := fmt.Sprintf(prompt, 
		tierPolicy.ModelsText,
		tierPolicy.DefaultReasoning,
		tierPolicy.Description,
		contextText,
		newUserMessage,
		trollingModelsText,
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

	model := a.config.AgentModelSelectionModel
	
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

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)
	
	if err != nil {
		log.E("Failed to get model selection", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_model_selection_failed", "reason", "api_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		gradeLimits := a.config.GradeLimits[userGrade]
		fallbackModel := gradeLimits.DialerModels[0]
		if len(gradeLimits.DialerModels) > 1 {
			fallbackModel = gradeLimits.DialerModels[1]
		}
		return &ModelSelectionResponse{
			RecommendedModel: fallbackModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Reasoning:        "Fallback to default settings due to agent failure",
		}, nil
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in model selection response")
		log.I("agent_model_selection_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		gradeLimits := a.config.GradeLimits[userGrade]
		fallbackModel := gradeLimits.DialerModels[0]
		if len(gradeLimits.DialerModels) > 1 {
			fallbackModel = gradeLimits.DialerModels[1]
		}
		return &ModelSelectionResponse{
			RecommendedModel: fallbackModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Reasoning:        "Fallback to default settings due to empty choices",
		}, nil
	}

	responseText := response.Choices[0].Message.Content.Text
	
	var modelResponse ModelSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &modelResponse); err != nil {
		log.E("Failed to parse model selection response", tracing.InnerError, err, "response_text", responseText)
		log.I("agent_model_selection_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		gradeLimits := a.config.GradeLimits[userGrade]
		fallbackModel := gradeLimits.DialerModels[0]
		if len(gradeLimits.DialerModels) > 1 {
			fallbackModel = gradeLimits.DialerModels[1]
		}
		return &ModelSelectionResponse{
			RecommendedModel: fallbackModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Reasoning:        "Fallback to default settings due to parsing failure",
		}, nil
	}

	// Validate and adjust based on tier policy
	originalModel := modelResponse.RecommendedModel
	originalEffort := modelResponse.ReasoningEffort
	modelResponse = a.validateModelSelection(modelResponse, userGrade)
	
	modelChanged := originalModel != modelResponse.RecommendedModel
	effortChanged := originalEffort != modelResponse.ReasoningEffort

	log.I("agent_model_selection_validated",
		"final_model", modelResponse.RecommendedModel,
		"final_reasoning_effort", modelResponse.ReasoningEffort,
		"model_changed", modelChanged,
		"effort_changed", effortChanged,
		"original_model", originalModel,
		"original_effort", originalEffort,
		"task_complexity", modelResponse.TaskComplexity,
		"requires_speed", modelResponse.RequiresSpeed,
		"requires_quality", modelResponse.RequiresQuality,
		"is_trolling", modelResponse.IsTrolling,
		"agent_reasoning", modelResponse.Reasoning,
		"duration_ms", duration.Milliseconds(),
		"user_grade", userGrade,
	)

	return &modelResponse, nil
}

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

// parseIndicesAndRanges parses index strings that can be either single indices or ranges
// Examples: "5" -> [5], "1-14" -> [1,2,3,...,14], "7-9" -> [7,8,9]
func (a *AgentSystem) parseIndicesAndRanges(indicesStrs []string, maxIndex int) ([]int, error) {
	var result []int
	seen := make(map[int]bool)
	
	for _, str := range indicesStrs {
		str = strings.TrimSpace(str)
		
		// Check if it's a range (contains "-")
		if strings.Contains(str, "-") {
			parts := strings.Split(str, "-")
			if len(parts) != 2 {
				continue // Invalid range format, skip
			}
			
			var start, end int
			if _, err := fmt.Sscanf(parts[0], "%d", &start); err != nil {
				continue // Invalid start, skip
			}
			if _, err := fmt.Sscanf(parts[1], "%d", &end); err != nil {
				continue // Invalid end, skip
			}
			
			// Validate range
			if start > end || start < 0 || end > maxIndex {
				continue // Invalid range, skip
			}
			
			// Add all indices in range
			for i := start; i <= end; i++ {
				if !seen[i] {
					result = append(result, i)
					seen[i] = true
				}
			}
		} else {
			// Single index
			var index int
			if _, err := fmt.Sscanf(str, "%d", &index); err != nil {
				continue // Invalid index, skip
			}
			
			if index >= 0 && index <= maxIndex && !seen[index] {
				result = append(result, index)
				seen[index] = true
			}
		}
	}
	
	return result, nil
}


type TierPolicy struct {
	ModelsText       string
	DefaultReasoning string
	Description      string
	DowngradeModels  []string
}

func (a *AgentSystem) getTierPolicy(userGrade platform.UserGrade) TierPolicy {
	gradeLimits, ok := a.config.GradeLimits[userGrade]
	if !ok {
		gradeLimits = a.config.GradeLimits[platform.GradeBronze]
	}

	switch userGrade {
	case platform.GradeGold:
		silverModels := a.config.GradeLimits[platform.GradeSilver].DialerModels
		bronzeModels := a.config.GradeLimits[platform.GradeBronze].DialerModels
		downgradeModels := append(silverModels, bronzeModels...)
		
		return TierPolicy{
			ModelsText:       strings.Join(gradeLimits.DialerModels, ", "),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Gold tier: maximum quality, deep reasoning, high reliability",
			DowngradeModels:  downgradeModels,
		}
	case platform.GradeSilver:
		bronzeModels := a.config.GradeLimits[platform.GradeBronze].DialerModels
		
		return TierPolicy{
			ModelsText:       strings.Join(gradeLimits.DialerModels, ", "),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Silver tier: balance quality and cost, low latency",
			DowngradeModels:  bronzeModels,
		}
	case platform.GradeBronze:
		return TierPolicy{
			ModelsText:       strings.Join(gradeLimits.DialerModels, ", "),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Bronze tier: minimal cost and latency",
			DowngradeModels:  []string{},
		}
	default:
		return TierPolicy{
			ModelsText:       strings.Join(gradeLimits.DialerModels, ", "),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Default tier",
			DowngradeModels:  []string{},
		}
	}
}

func (a *AgentSystem) validateModelSelection(response ModelSelectionResponse, userGrade platform.UserGrade) ModelSelectionResponse {
	tierPolicy := a.getTierPolicy(userGrade)
	gradeLimits := a.config.GradeLimits[userGrade]
	
	// Check if recommended model is available for this tier or in trolling models
	modelValid := false
	
	// Check tier models first
	for _, model := range gradeLimits.DialerModels {
		if model == response.RecommendedModel {
			modelValid = true
			break
		}
	}
	
	// Check downgrade models
	if !modelValid {
		for _, model := range tierPolicy.DowngradeModels {
			if model == response.RecommendedModel {
				modelValid = true
				break
			}
		}
	}
	
	// Check trolling models (regardless of is_trolling flag)
	if !modelValid {
		for _, model := range a.config.TrollingModels {
			if model == response.RecommendedModel {
				modelValid = true
				// If LLM chose trolling model, mark as trolling
				response.IsTrolling = true
				break
			}
		}
	}
	
	// Fallback if model not valid
	if !modelValid {
		if response.IsTrolling && len(a.config.TrollingModels) > 0 {
			response.RecommendedModel = a.config.TrollingModels[0]
		} else if len(gradeLimits.DialerModels) > 1 {
			response.RecommendedModel = gradeLimits.DialerModels[1]
		} else if len(gradeLimits.DialerModels) > 0 {
			response.RecommendedModel = gradeLimits.DialerModels[0]
		}
	}
	
	// Set low reasoning for trolling
	if response.IsTrolling {
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