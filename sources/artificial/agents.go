package artificial

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	openrouter "github.com/revrost/go-openrouter"
)

// ContextSelectionResponse represents the response from context selection agent
type ContextSelectionResponse struct {
	RelevantIndices []string `json:"relevant_indices"`
}

// ModelSelectionResponse represents the response from model selection agent
type ModelSelectionResponse struct {
	RecommendedModel    string  `json:"recommended_model"`
	ReasoningEffort     string  `json:"reasoning_effort"`
	TaskComplexity      string  `json:"task_complexity"`
	RequiresSpeed       bool    `json:"requires_speed"`
	RequiresQuality     bool    `json:"requires_quality"`
	IsTrolling          bool    `json:"is_trolling"`
	Temperature         float32 `json:"temperature"`
}

// PersonalizationValidationResponse represents the response from personalization validation agent
type PersonalizationValidationResponse struct {
	Confidence float64 `json:"confidence"`
}

// ResponseLengthResponse represents the response from response length detection agent
type ResponseLengthResponse struct {
	Length     string  `json:"length"`     // very_brief, brief, medium, detailed, very_detailed
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reasoning  string  `json:"reasoning"`  // Brief explanation
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

	indices := texting.ExpandIndicesAndRanges(log, contextResponse.RelevantIndices, len(history)-1)
	
	// Ensure we select messages in pairs (user-assistant)
	// Convert indices to a set for faster lookup
	indicesSet := make(map[int]bool)
	for _, idx := range indices {
		indicesSet[idx] = true
	}
	
	// Add missing pairs
	for idx := range indicesSet {
		if idx >= 0 && idx < len(history) {
			msg := history[idx]
			
			// If this is a user message, ensure we have the following assistant message
			if msg.Role == platform.MessageRoleUser && idx+1 < len(history) {
				if history[idx+1].Role == platform.MessageRoleAssistant {
					indicesSet[idx+1] = true
				}
			}
			
			// If this is an assistant message, ensure we have the preceding user message
			if msg.Role == platform.MessageRoleAssistant && idx > 0 {
				if history[idx-1].Role == platform.MessageRoleUser {
					indicesSet[idx-1] = true
				}
			}
		}
	}
	
	// Convert back to sorted slice
	var completedIndices []int
	for idx := range indicesSet {
		completedIndices = append(completedIndices, idx)
	}
	
	// Sort indices
	for i := 0; i < len(completedIndices); i++ {
		for j := i + 1; j < len(completedIndices); j++ {
			if completedIndices[i] > completedIndices[j] {
				completedIndices[i], completedIndices[j] = completedIndices[j], completedIndices[i]
			}
		}
	}

	relevantMessages := []platform.RedisMessage{}
	for _, index := range completedIndices {
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
	
	// Build downgrade models text based on user grade
	downgradeModelsText := ""
	switch userGrade {
	case platform.GradeGold:
		silverModels := a.config.GradeLimits[platform.GradeSilver].DialerModels
		bronzeModels := a.config.GradeLimits[platform.GradeBronze].DialerModels
		downgradeModelsText = fmt.Sprintf("\n\nDowngrade models (use only for simple/fast tasks when tier models are overkill):\n\nSilver tier models:\n%s\n\nBronze tier models:\n%s", 
			formatModelsForPrompt(silverModels), 
			formatModelsForPrompt(bronzeModels))
	case platform.GradeSilver:
		bronzeModels := a.config.GradeLimits[platform.GradeBronze].DialerModels
		downgradeModelsText = fmt.Sprintf("\n\nDowngrade models (use only for simple/fast tasks when tier models are overkill):\n\nBronze tier models:\n%s", 
			formatModelsForPrompt(bronzeModels))
	case platform.GradeBronze:
		downgradeModelsText = ""
	}
	
	systemMessage := fmt.Sprintf(prompt, 
		tierPolicy.ModelsText,
		tierPolicy.DefaultReasoning,
		tierPolicy.Description,
		downgradeModelsText,
		trollingModelsText,
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

	model := a.config.AgentModelSelectionModel
	
	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
	}

	log = log.With("ai_agent", "model_selector", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)
	
	if err != nil {
		log.E("Failed to get model selection", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_model_selection_failed", "reason", "api_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		gradeLimits := a.config.GradeLimits[userGrade]
		fallbackModel := gradeLimits.DialerModels[0].Name
		if len(gradeLimits.DialerModels) > 1 {
			fallbackModel = gradeLimits.DialerModels[1].Name
		}
		return &ModelSelectionResponse{
			RecommendedModel: fallbackModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Temperature:      1.0,
		}, nil
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in model selection response")
		log.I("agent_model_selection_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		gradeLimits := a.config.GradeLimits[userGrade]
		fallbackModel := gradeLimits.DialerModels[0].Name
		if len(gradeLimits.DialerModels) > 1 {
			fallbackModel = gradeLimits.DialerModels[1].Name
		}
		return &ModelSelectionResponse{
			RecommendedModel: fallbackModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Temperature:      1.0,
		}, nil
	}

	responseText := response.Choices[0].Message.Content.Text
	
	var modelResponse ModelSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &modelResponse); err != nil {
		log.E("Failed to parse model selection response", tracing.InnerError, err, "response_text", responseText)
		log.I("agent_model_selection_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		gradeLimits := a.config.GradeLimits[userGrade]
		fallbackModel := gradeLimits.DialerModels[0].Name
		if len(gradeLimits.DialerModels) > 1 {
			fallbackModel = gradeLimits.DialerModels[1].Name
		}
		return &ModelSelectionResponse{
			RecommendedModel: fallbackModel,
			ReasoningEffort:  gradeLimits.DialerReasoningEffort,
			TaskComplexity:   "medium",
			RequiresSpeed:    false,
			RequiresQuality:  true,
			IsTrolling:       false,
			Temperature:      1.0,
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
		"final_temperature", modelResponse.Temperature,
		"model_changed", modelChanged,
		"effort_changed", effortChanged,
		"original_model", originalModel,
		"original_effort", originalEffort,
		"task_complexity", modelResponse.TaskComplexity,
		"requires_speed", modelResponse.RequiresSpeed,
		"requires_quality", modelResponse.RequiresQuality,
		"is_trolling", modelResponse.IsTrolling,
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

type TierPolicy struct {
	ModelsText       string
	DefaultReasoning string
	Description      string
	DowngradeModels  []ModelMeta
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
			ModelsText:       formatModelsForPrompt(gradeLimits.DialerModels),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Gold tier: maximum quality, deep reasoning, high reliability",
			DowngradeModels:  downgradeModels,
		}
	case platform.GradeSilver:
		bronzeModels := a.config.GradeLimits[platform.GradeBronze].DialerModels
		
		return TierPolicy{
			ModelsText:       formatModelsForPrompt(gradeLimits.DialerModels),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Silver tier: balance quality and cost, low latency",
			DowngradeModels:  bronzeModels,
		}
	case platform.GradeBronze:
		return TierPolicy{
			ModelsText:       formatModelsForPrompt(gradeLimits.DialerModels),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Bronze tier: minimal cost and latency",
			DowngradeModels:  []ModelMeta{},
		}
	default:
		return TierPolicy{
			ModelsText:       formatModelsForPrompt(gradeLimits.DialerModels),
			DefaultReasoning: gradeLimits.DialerReasoningEffort,
			Description:      "Unknown tier",
			DowngradeModels:  []ModelMeta{},
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
		if model.Name == response.RecommendedModel {
			modelValid = true
			break
		}
	}
	
	// Check downgrade models
	if !modelValid {
		for _, model := range tierPolicy.DowngradeModels {
			if model.Name == response.RecommendedModel {
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
			response.RecommendedModel = gradeLimits.DialerModels[1].Name
		} else if len(gradeLimits.DialerModels) > 0 {
			response.RecommendedModel = gradeLimits.DialerModels[0].Name
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

func (a *AgentSystem) ValidatePersonalization(
	log *tracing.Logger,
	text string,
) (*PersonalizationValidationResponse, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
	defer cancel()

	prompt := `You are a validation agent. Your task is to determine if the provided text is a self-description or personal information about the user.

Examples of valid self-descriptions:
- "I am a software engineer from Russia, I love coding and hiking"
- "My name is Ivan, I'm 25 years old student"
- "I work as a designer, passionate about art and music"
- "I'm a teacher who loves reading books and traveling"

Examples of invalid texts:
- "How to cook pasta?" (question, not self-description)
- "The weather is nice today" (general statement)
- "Buy groceries tomorrow" (task/reminder)
- "Python is a great language" (opinion about something else)

Analyze the following text and determine if it's a valid self-description.
Return your response in JSON format: {"confidence": 0.0-1.0}
Where confidence is how certain you are that this is a self-description (1.0 = definitely self-description, 0.0 = definitely not).

Text to analyze: %s

Return ONLY JSON, nothing else.`

	systemMessage := fmt.Sprintf(prompt, text)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Analyze the text and return the confidence score in JSON format."},
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

	log = log.With("ai_agent", "personalization_validator", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to validate personalization", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		return nil, err
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in personalization validation response")
		return nil, fmt.Errorf("empty response from validation agent")
	}

	responseText := response.Choices[0].Message.Content.Text

	var validationResponse PersonalizationValidationResponse
	if err := json.Unmarshal([]byte(responseText), &validationResponse); err != nil {
		log.E("Failed to parse personalization validation response", tracing.InnerError, err, "response_text", responseText)
		return nil, fmt.Errorf("failed to parse validation response: %w", err)
	}

	log.I("personalization_validation_success",
		"confidence", validationResponse.Confidence,
		"duration_ms", duration.Milliseconds(),
	)

	return &validationResponse, nil
}

// SummarizeContent uses an agent to summarize conversation content
func (a *AgentSystem) SummarizeContent(
	log *tracing.Logger,
	content string,
	contentType string, // "message" or "cluster"
) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.SummarizationTimeout)*time.Second)
	defer cancel()

	prompt := a.getSummarizationPrompt()
	systemMessage := fmt.Sprintf(prompt, content)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Summarize the provided content according to the requirements. Return only the summary text."},
		},
	}

	model := a.config.SummarizationModel

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.3, // Lower temperature for consistent summarization
	}

	log = log.With("ai_agent", "summarizer", tracing.AiModel, model, "content_type", contentType)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to summarize content", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_summarization_failed", "reason", "api_error", "duration_ms", duration.Milliseconds(), "content_type", contentType)
		// Return original content if summarization fails
		return content, fmt.Errorf("summarization failed: %w", err)
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in summarization response")
		log.I("agent_summarization_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "content_type", contentType)
		return content, fmt.Errorf("empty response from summarization agent")
	}

	summarizedText := response.Choices[0].Message.Content.Text
	
	originalLength := len(content)
	summarizedLength := len(summarizedText)
	reductionPercent := 0
	if originalLength > 0 {
		reductionPercent = int(float64(originalLength-summarizedLength) / float64(originalLength) * 100)
	}

	log.I("agent_summarization_success",
		"original_length", originalLength,
		"summarized_length", summarizedLength,
		"reduction_percent", reductionPercent,
		"duration_ms", duration.Milliseconds(),
		"content_type", contentType,
	)

	return summarizedText, nil
}

// Prompt getters - these are loaded from configuration with base64 decoding
func (a *AgentSystem) getContextSelectionPrompt() string {
	return a.config.GetContextSelectionPrompt()
}

func (a *AgentSystem) getModelSelectionPrompt() string {
	return a.config.GetModelSelectionPrompt()
}

func (a *AgentSystem) getSummarizationPrompt() string {
	return a.config.GetSummarizationPrompt()
}

func (a *AgentSystem) getResponseLengthPrompt() string {
	return a.config.GetResponseLengthPrompt()
}

func (a *AgentSystem) DetermineResponseLength(
	log *tracing.Logger,
	userMessage string,
) (*ResponseLengthResponse, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AgentResponseLengthTimeout)*time.Second)
	defer cancel()

	prompt := a.getResponseLengthPrompt()
	systemMessage := fmt.Sprintf(prompt, userMessage)

	request := openrouter.ChatCompletionRequest{
		Model: a.config.AgentResponseLengthModel,
		Messages: []openrouter.ChatCompletionMessage{
			{Role: openrouter.ChatMessageRoleSystem, Content: openrouter.Content{Text: systemMessage}},
			{Role: openrouter.ChatMessageRoleUser, Content: openrouter.Content{Text: "Analyze the user's message and determine the expected response length. Return your response in JSON format."}},
		},
		Provider:    &openrouter.ChatProvider{DataCollection: openrouter.DataCollectionDeny, Sort: openrouter.ProviderSortingLatency},
		Temperature: 0.2,
	}

	log = log.With("ai_agent", "response_length_detector", tracing.AiModel, a.config.AgentResponseLengthModel)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to determine response length", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_response_length_failed", "reason", "api_error", "duration_ms", duration.Milliseconds())
		return a.getDefaultResponseLength("API error"), nil
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in response length detection")
		log.I("agent_response_length_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds())
		return a.getDefaultResponseLength("empty response"), nil
	}

	var lengthResponse ResponseLengthResponse
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content.Text), &lengthResponse); err != nil {
		log.E("Failed to parse response length detection", tracing.InnerError, err, "response_text", response.Choices[0].Message.Content.Text)
		log.I("agent_response_length_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds())
		return a.getDefaultResponseLength("parse error"), nil
	}

	if !a.isValidResponseLength(lengthResponse.Length) {
		log.W("Invalid length value returned, defaulting to medium", "returned_length", lengthResponse.Length)
		lengthResponse.Length = "medium"
		lengthResponse.Confidence = 0.5
	}

	log.I("agent_response_length_success",
		"detected_length", lengthResponse.Length,
		"confidence", lengthResponse.Confidence,
		"reasoning", lengthResponse.Reasoning,
		"duration_ms", duration.Milliseconds(),
	)

	return &lengthResponse, nil
}

func (a *AgentSystem) getDefaultResponseLength(reason string) *ResponseLengthResponse {
	return &ResponseLengthResponse{
		Length:     "medium",
		Confidence: 0.5,
		Reasoning:  fmt.Sprintf("Defaulted to medium due to %s", reason),
	}
}

func (a *AgentSystem) isValidResponseLength(length string) bool {
	validLengths := map[string]bool{
		"very_brief": true, "brief": true, "medium": true,
		"detailed": true, "very_detailed": true,
	}
	return validLengths[length]
}