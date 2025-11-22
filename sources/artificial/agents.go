package artificial

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/indices"
	"ximanager/sources/tracing"

	openrouter "github.com/revrost/go-openrouter"
)

// ContextSelectionResponse represents the response from context selection agent
type ContextSelectionResponse struct {
	RelevantIndices []string `json:"relevant_indices"`
}

// ModelSelectionResponse represents the response from model selection agent
type ModelSelectionResponse struct {
	RecommendedModel string  `json:"recommended_model"`
	ReasoningEffort  string  `json:"reasoning_effort"`
	TaskComplexity   string  `json:"task_complexity"`
	RequiresSpeed    bool    `json:"requires_speed"`
	RequiresQuality  bool    `json:"requires_quality"`
	IsTrolling       bool    `json:"is_trolling"`
	Temperature      float32 `json:"temperature"`
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
	ai      *openrouter.Client
	config  *configuration.Config
	tariffs repository.Tariffs
}

type AgentUsageAccumulator struct {
	TotalTokens int
	Cost        float64
}

func (a *AgentUsageAccumulator) Add(tokens int, cost float64) {
	a.TotalTokens += tokens
	a.Cost += cost
}

func NewAgentSystem(ai *openrouter.Client, config *configuration.Config, tariffs repository.Tariffs) *AgentSystem {
	return &AgentSystem{
		ai:      ai,
		config:  config,
		tariffs: tariffs,
	}
}

// SelectRelevantContext uses an agent to determine which messages from history are relevant
func (a *AgentSystem) SelectRelevantContext(
	log *tracing.Logger,
	history []platform.RedisMessage,
	newUserMessage string,
	userGrade platform.UserGrade,
	agentUsage *AgentUsageAccumulator,
) ([]platform.RedisMessage, error) {
	defer tracing.ProfilePoint(log, "Agent select relevant context completed", "artificial.agents.select.relevant.context", "history_count", len(history), "user_grade", userGrade)()

	if len(history) <= 4 {
		return history, nil
	}

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.Context.Timeout)*time.Second)
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

	model := a.config.AI.Agents.Context.Model

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
		Usage:       &openrouter.IncludeUsage{Include: true},
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

	if agentUsage != nil {
		agentUsage.Add(response.Usage.TotalTokens, response.Usage.Cost)
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in context selection response")
		log.I("agent_context_selection_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return history, nil
	}

	responseText := cleanJSONFromMarkdown(response.Choices[0].Message.Content.Text)

	var contextResponse ContextSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &contextResponse); err != nil {
		log.E("Failed to parse context selection response", tracing.InnerError, err, "response_text", responseText)
		log.I("agent_context_selection_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return history, nil
	}

	indices := indices.Expand(log, contextResponse.RelevantIndices, len(history)-1)

	// Ensure we select messages in pairs (user-assistant)
	// Convert indices to a set for faster lookup
	indicesSet := make(map[int]bool)
	for _, idx := range indices {
		indicesSet[idx] = true
	}

	// Add missing pairs - iterate over copy to avoid modifying map during iteration
	currentIndices := make([]int, 0, len(indicesSet))
	for idx := range indicesSet {
		currentIndices = append(currentIndices, idx)
	}

	for _, idx := range currentIndices {
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
	sort.Ints(completedIndices)

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
	agentUsage *AgentUsageAccumulator,
) (*ModelSelectionResponse, error) {
	defer tracing.ProfilePoint(log, "Agent select model and effort completed", "artificial.agents.select.model.and.effort", "context_count", len(selectedContext), "user_grade", userGrade)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.ModelSelection.Timeout)*time.Second)
	defer cancel()

	// Get tier policy for the user
	tierPolicy, err := a.getTierPolicy(ctx, userGrade)
	if err != nil {
		log.E("Failed to get tier policy", tracing.InnerError, err)
		// Cannot proceed without tariff info, but maybe we can fallback?
		// For now, return error or fallback to minimal.
		// I'll fallback to a safe default manually if possible, but getTierPolicy tries fallback.
		return nil, err
	}

	// Limit context to last 6 messages (3 pairs) to save tokens
	limitedContext := selectedContext
	if len(selectedContext) > 6 {
		limitedContext = selectedContext[len(selectedContext)-6:]
	}
	contextText := a.formatHistoryForAgent(limitedContext)

	prompt := a.getModelSelectionPrompt()

	trollingModelsText := strings.Join(a.config.AI.PlaceboModels, ", ")

	// Build downgrade models text based on user grade
	// This requires knowing lower tier models.
	// getTierPolicy now returns DowngradeModels.
	downgradeModelsText := ""
	if len(tierPolicy.DowngradeModels) > 0 {
		downgradeModelsText = fmt.Sprintf("\n\nDowngrade models (use only for simple/fast tasks when tier models are overkill):\n\nLower tier models:\n%s",
			formatModelsForPrompt(tierPolicy.DowngradeModels))
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

	model := a.config.AI.Agents.ModelSelection.Model

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
		Usage:       &openrouter.IncludeUsage{Include: true},
	}

	log = log.With("ai_agent", "model_selector", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to get model selection", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_model_selection_failed", "reason", "api_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return a.getModelSelectionFallback(tierPolicy), nil
	}

	if agentUsage != nil {
		agentUsage.Add(response.Usage.TotalTokens, response.Usage.Cost)
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in model selection response")
		log.I("agent_model_selection_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return a.getModelSelectionFallback(tierPolicy), nil
	}

	responseText := cleanJSONFromMarkdown(response.Choices[0].Message.Content.Text)

	var modelResponse ModelSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &modelResponse); err != nil {
		log.E("Failed to parse model selection response", tracing.InnerError, err, "response_text", responseText)
		log.I("agent_model_selection_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return a.getModelSelectionFallback(tierPolicy), nil
	}

	// Validate and adjust based on tier policy
	originalModel := modelResponse.RecommendedModel
	originalEffort := modelResponse.ReasoningEffort
	modelResponse = a.validateModelSelection(modelResponse, tierPolicy, userGrade)

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

func (a *AgentSystem) getModelSelectionFallback(tierPolicy TierPolicy) *ModelSelectionResponse {
	fallbackModel := ""
	if len(tierPolicy.Models) > 0 {
		fallbackModel = tierPolicy.Models[0].Name
		if len(tierPolicy.Models) > 1 {
			fallbackModel = tierPolicy.Models[1].Name
		}
	}

	return &ModelSelectionResponse{
		RecommendedModel: fallbackModel,
		ReasoningEffort:  tierPolicy.DefaultReasoning,
		TaskComplexity:   "medium",
		RequiresSpeed:    false,
		RequiresQuality:  true,
		IsTrolling:       false,
		Temperature:      1.0,
	}
}

func cleanJSONFromMarkdown(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

type TierPolicy struct {
	Models           []ModelMeta
	ModelsText       string
	DefaultReasoning string
	Description      string
	DowngradeModels  []ModelMeta
}

func (a *AgentSystem) getTierPolicy(ctx context.Context, userGrade platform.UserGrade) (TierPolicy, error) {
	// 1. Fetch current tier tariff
	currentTariff, err := a.tariffs.GetLatestByKey(ctx, string(userGrade))
	if err != nil {
		// Try fallback to bronze if not bronze
		if userGrade != platform.GradeBronze {
			currentTariff, err = a.tariffs.GetLatestByKey(ctx, string(platform.GradeBronze))
		}
		if err != nil {
			return TierPolicy{}, fmt.Errorf("failed to fetch tariff: %w", err)
		}
	}

	var currentModels []ModelMeta
	if err := json.Unmarshal(currentTariff.DialerModels, &currentModels); err != nil {
		return TierPolicy{}, fmt.Errorf("failed to unmarshal models: %w", err)
	}

	policy := TierPolicy{
		Models:           currentModels,
		ModelsText:       formatModelsForPrompt(currentModels),
		DefaultReasoning: currentTariff.DialerReasoningEffort,
		Description:      fmt.Sprintf("%s tier: %s", userGrade, currentTariff.DisplayName),
	}

	// 2. Fetch downgrade models
	var downgradeModels []ModelMeta
	if userGrade == platform.GradeGold {
		// Need Silver + Bronze
		silverTariff, _ := a.tariffs.GetLatestByKey(ctx, string(platform.GradeSilver))
		if silverTariff != nil {
			var silverModels []ModelMeta
			_ = json.Unmarshal(silverTariff.DialerModels, &silverModels)
			downgradeModels = append(downgradeModels, silverModels...)
		}
		bronzeTariff, _ := a.tariffs.GetLatestByKey(ctx, string(platform.GradeBronze))
		if bronzeTariff != nil {
			var bronzeModels []ModelMeta
			_ = json.Unmarshal(bronzeTariff.DialerModels, &bronzeModels)
			downgradeModels = append(downgradeModels, bronzeModels...)
		}
	} else if userGrade == platform.GradeSilver {
		// Need Bronze
		bronzeTariff, _ := a.tariffs.GetLatestByKey(ctx, string(platform.GradeBronze))
		if bronzeTariff != nil {
			var bronzeModels []ModelMeta
			_ = json.Unmarshal(bronzeTariff.DialerModels, &bronzeModels)
			downgradeModels = append(downgradeModels, bronzeModels...)
		}
	}

	policy.DowngradeModels = downgradeModels
	return policy, nil
}

func (a *AgentSystem) validateModelSelection(response ModelSelectionResponse, tierPolicy TierPolicy, userGrade platform.UserGrade) ModelSelectionResponse {
	// Check if recommended model is available for this tier or in trolling models
	modelValid := false

	// Check tier models first
	for _, model := range tierPolicy.Models {
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
		for _, model := range a.config.AI.PlaceboModels {
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
		if response.IsTrolling && len(a.config.AI.PlaceboModels) > 0 {
			response.RecommendedModel = a.config.AI.PlaceboModels[0]
		} else if len(tierPolicy.Models) > 1 {
			response.RecommendedModel = tierPolicy.Models[1].Name
		} else if len(tierPolicy.Models) > 0 {
			response.RecommendedModel = tierPolicy.Models[0].Name
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
	defer tracing.ProfilePoint(log, "Agent validate personalization completed", "artificial.agents.validate.personalization", "text_length", len(text))()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
	defer cancel()

	prompt := a.getPersonalizationValidationPrompt()
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

	model := a.config.AI.Agents.Context.Model

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

	responseText := cleanJSONFromMarkdown(response.Choices[0].Message.Content.Text)

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
	defer tracing.ProfilePoint(log, "Agent summarize content completed", "artificial.agents.summarize.content", "content_type", contentType, "content_length", len(content))()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.Summarization.Timeout)*time.Second)
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

	model := a.config.AI.Agents.Summarization.Model

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

// Prompt getters
func (a *AgentSystem) getContextSelectionPrompt() string {
	p := a.config.AI.Prompts.ContextSelection
	if p == "" {
		return getDefaultContextSelectionPrompt()
	}
	return decodePrompt(p, getDefaultContextSelectionPrompt())
}

func (a *AgentSystem) getModelSelectionPrompt() string {
	p := a.config.AI.Prompts.ModelSelection
	if p == "" {
		return getDefaultModelSelectionPrompt()
	}
	return decodePrompt(p, getDefaultModelSelectionPrompt())
}

func (a *AgentSystem) getSummarizationPrompt() string {
	p := a.config.AI.Prompts.Summarization
	if p == "" {
		return getDefaultSummarizationPrompt()
	}
	return decodePrompt(p, getDefaultSummarizationPrompt())
}

func (a *AgentSystem) getResponseLengthPrompt() string {
	p := a.config.AI.Prompts.ResponseLength
	if p == "" {
		return getDefaultResponseLengthPrompt()
	}
	return decodePrompt(p, getDefaultResponseLengthPrompt())
}

func (a *AgentSystem) getPersonalizationValidationPrompt() string {
	p := a.config.AI.Prompts.PersonalizationValidation
	if p == "" {
		return getDefaultPersonalizationValidationPrompt()
	}
	return decodePrompt(p, getDefaultPersonalizationValidationPrompt())
}

func decodePrompt(raw, fallback string) string {
	// Try base64 decode
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return string(decoded)
	}
	// If failed, assume plain text
	return raw
}

func (a *AgentSystem) DetermineResponseLength(
	log *tracing.Logger,
	userMessage string,
	agentUsage *AgentUsageAccumulator,
) (*ResponseLengthResponse, error) {
	defer tracing.ProfilePoint(log, "Agent determine response length completed", "artificial.agents.determine.response.length")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.ResponseLength.Timeout)*time.Second)
	defer cancel()

	prompt := a.getResponseLengthPrompt()
	systemMessage := fmt.Sprintf(prompt, userMessage)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Analyze the user message and determine the appropriate response length. Return your response in JSON format."},
		},
	}

	model := a.config.AI.Agents.ResponseLength.Model

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.2,
		Usage:       &openrouter.IncludeUsage{Include: true},
	}

	log = log.With("ai_agent", "response_length_detector", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to determine response length", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_response_length_failed", "reason", "api_error", "duration_ms", duration.Milliseconds())
		return a.getDefaultResponseLength("API error"), nil
	}

	if agentUsage != nil {
		agentUsage.Add(response.Usage.TotalTokens, response.Usage.Cost)
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in response length detection")
		log.I("agent_response_length_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds())
		return a.getDefaultResponseLength("empty response"), nil
	}

	responseText := cleanJSONFromMarkdown(response.Choices[0].Message.Content.Text)
	
	var lengthResponse ResponseLengthResponse
	if err := json.Unmarshal([]byte(responseText), &lengthResponse); err != nil {
		log.E("Failed to parse response length detection", tracing.InnerError, err)
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

// Helper to format models for prompt (copied from config.go logic)
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