package artificial

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/metrics"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	openrouter "github.com/revrost/go-openrouter"
)

// EffortSelectionResponse represents the response from effort selection agent
type EffortSelectionResponse struct {
	ReasoningEffort string  `json:"reasoning_effort"`
	TaskComplexity  string  `json:"task_complexity"`
	RequiresSpeed   bool    `json:"requires_speed"`
	RequiresQuality bool    `json:"requires_quality"`
	Temperature     float32 `json:"temperature"`
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

// WebSearchResponse represents the response from web search agent
type WebSearchResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// PersonalizationExtractionResponse represents the response from personalization extraction agent
type PersonalizationExtractionResponse struct {
	HasNewInfo     bool     `json:"has_new_info"`
	NewFacts       []string `json:"new_facts"`
	UpdatedProfile *string  `json:"updated_profile"`
}

// AgentSystem handles the agent-based AI workflow
type AgentSystem struct {
	ai      *openrouter.Client
	config  *configuration.Config
	tariffs *repository.TariffsRepository
	metrics *metrics.MetricsService
	log     *tracing.Logger
}

type AgentUsageAccumulator struct {
	mu          sync.Mutex
	totalTokens int
	cost        float64
}

func (a *AgentUsageAccumulator) Add(tokens int, cost float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.totalTokens += tokens
	a.cost += cost
}

func (a *AgentUsageAccumulator) GetTotalTokens() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totalTokens
}

func (a *AgentUsageAccumulator) GetCost() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cost
}

func NewAgentSystem(ai *openrouter.Client, config *configuration.Config, tariffs *repository.TariffsRepository, metrics *metrics.MetricsService, log *tracing.Logger) *AgentSystem {
	return &AgentSystem{
		ai:      ai,
		config:  config,
		tariffs: tariffs,
		metrics: metrics,
		log:     log,
	}
}

// SelectEffort uses an agent to determine the appropriate reasoning effort
func (a *AgentSystem) SelectEffort(
	log *tracing.Logger,
	selectedContext []platform.RedisMessage,
	newUserMessage string,
	userGrade platform.UserGrade,
	agentUsage *AgentUsageAccumulator,
) (*EffortSelectionResponse, error) {
	defer tracing.ProfilePoint(log, "Agent select effort completed", "artificial.agents.select.effort", "context_count", len(selectedContext), "user_grade", userGrade)()
	a.metrics.RecordAgentUsage("effort_selection")

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.EffortSelection.Timeout)*time.Second)
	defer cancel()

	limitedContext := selectedContext
	if len(selectedContext) > 6 {
		limitedContext = selectedContext[len(selectedContext)-6:]
	}
	contextText := a.formatHistoryForAgent(limitedContext)

	prompt := a.getEffortSelectionPrompt()

	systemMessage := fmt.Sprintf(prompt, "", contextText, newUserMessage)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Analyze the task and recommend the appropriate reasoning effort. Return your response in the specified JSON format."},
		},
	}

	model := a.config.AI.Agents.EffortSelection.Model

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
		Usage:       &openrouter.IncludeUsage{Include: true},
		Transforms:  []string{},
	}

	log = log.With("ai_agent", "effort_selector", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	fallback := &EffortSelectionResponse{
		ReasoningEffort: "medium",
		TaskComplexity:  "medium",
		RequiresSpeed:   false,
		RequiresQuality: true,
		Temperature:     1.0,
	}

	if err != nil {
		log.E("Failed to get effort selection", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_effort_selection_failed", "reason", "api_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return fallback, nil
	}

	if agentUsage != nil {
		agentUsage.Add(response.Usage.TotalTokens, response.Usage.Cost)
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in effort selection response")
		log.I("agent_effort_selection_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return fallback, nil
	}

	responseText := cleanJSONFromMarkdown(response.Choices[0].Message.Content.Text)

	var effortResponse EffortSelectionResponse
	if err := json.Unmarshal([]byte(responseText), &effortResponse); err != nil {
		log.E("Failed to parse effort selection response", tracing.InnerError, err, "response_text", responseText)
		log.I("agent_effort_selection_failed", "reason", "json_parse_error", "duration_ms", duration.Milliseconds(), "user_grade", userGrade)
		return fallback, nil
	}

	log.I("agent_effort_selection_success",
		"reasoning_effort", effortResponse.ReasoningEffort,
		"task_complexity", effortResponse.TaskComplexity,
		"temperature", effortResponse.Temperature,
		"requires_speed", effortResponse.RequiresSpeed,
		"requires_quality", effortResponse.RequiresQuality,
		"duration_ms", duration.Milliseconds(),
		"user_grade", userGrade,
	)

	return &effortResponse, nil
}

func (a *AgentSystem) formatHistoryForAgent(history []platform.RedisMessage) string {
	var parts []string
	for i, msg := range history {
		role := "User"
		switch msg.Role {
		case platform.MessageRoleAssistant:
			role = "Assistant"
		case platform.MessageRoleSystem:
			role = "System"
		}
		parts = append(parts, fmt.Sprintf("[%d] %s: %s", i, role, msg.Content))
	}
	return strings.Join(parts, "\n")
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

func (a *AgentSystem) ValidatePersonalization(
	log *tracing.Logger,
	text string,
) (*PersonalizationValidationResponse, error) {
	defer tracing.ProfilePoint(log, "Agent validate personalization completed", "artificial.agents.validate.personalization", "text_length", len(text))()
	a.metrics.RecordAgentUsage("personalization_validation")
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

	model := a.config.AI.Agents.EffortSelection.Model

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
		Transforms:  []string{},
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
	a.metrics.RecordAgentUsage("summarization")
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
		Transforms:  []string{},
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
func (a *AgentSystem) getEffortSelectionPrompt() string {
	p := a.config.AI.Prompts.EffortSelection
	if p == "" {
		return getDefaultEffortSelectionPrompt()
	}
	return decodePrompt(p, getDefaultEffortSelectionPrompt())
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

func (a *AgentSystem) getWebSearchPrompt() string {
	p := a.config.AI.Prompts.WebSearch
	if p == "" {
		return getDefaultWebSearchPrompt()
	}
	return decodePrompt(p, getDefaultWebSearchPrompt())
}

func (a *AgentSystem) getPersonalizationExtractionPrompt() string {
	p := a.config.AI.Prompts.PersonalizationExtraction
	if p == "" {
		return getDefaultPersonalizationExtractionPrompt()
	}
	return decodePrompt(p, getDefaultPersonalizationExtractionPrompt())
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
	a.metrics.RecordAgentUsage("response_length")
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
		Transforms:  []string{},
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

func (a *AgentSystem) WebSearch(
	log *tracing.Logger,
	query string,
	isDeepSearch bool,
	effortOverride string,
	agentUsage *AgentUsageAccumulator,
) (*WebSearchResponse, error) {
	defer tracing.ProfilePoint(log, "Agent web search completed", "artificial.agents.web.search", "query_length", len(query), "is_deep", isDeepSearch)()
	a.metrics.RecordAgentUsage("web_search")

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.WebSearch.Timeout)*time.Second)
	defer cancel()

	prompt := a.getWebSearchPrompt()
	systemMessage := fmt.Sprintf(prompt, query)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Search the internet for the most relevant and recent information related to the query."},
		},
	}

	model := a.config.AI.Agents.WebSearch.BriefModel
	if isDeepSearch {
		model = a.config.AI.Agents.WebSearch.DeepModel
	}

	effort := a.config.AI.Agents.WebSearch.ReasoningEffort
	if effortOverride != "" && (effortOverride == "low" || effortOverride == "medium" || effortOverride == "high") {
		effort = effortOverride
	}

	searchContextSize := openrouter.SearchContextSizeMedium
	switch effort {
	case "low":
		searchContextSize = openrouter.SearchContextSizeLow
	case "high":
		searchContextSize = openrouter.SearchContextSizeHigh
	}

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.3,
		Usage:       &openrouter.IncludeUsage{Include: true},
		WebSearchOptions: &openrouter.WebSearchOptions{
			SearchContextSize: searchContextSize,
		},
		Transforms: []string{},
	}

	log = log.With("ai_agent", "web_search", tracing.AiModel, model, "effort", effort, "is_deep", isDeepSearch)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to perform web search", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		log.I("agent_web_search_failed", "reason", "api_error", "duration_ms", duration.Milliseconds())

		model = a.config.AI.Agents.WebSearch.FallbackModel
		request.Model = model
		log = log.With(tracing.AiModel, model)

		response, err = a.ai.CreateChatCompletion(ctx, request)
		if err != nil {
			log.E("Fallback web search also failed", tracing.InnerError, err)
			return &WebSearchResponse{
				Result: "",
				Error:  "Web search failed: unable to retrieve information at this time. Please try again later.",
			}, nil
		}
	}

	if agentUsage != nil {
		agentUsage.Add(response.Usage.TotalTokens, response.Usage.Cost)
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in web search response")
		log.I("agent_web_search_failed", "reason", "empty_choices", "duration_ms", duration.Milliseconds())
		return &WebSearchResponse{
			Result: "",
			Error:  "Web search returned no results.",
		}, nil
	}

	resultText := response.Choices[0].Message.Content.Text

	log.I("agent_web_search_success",
		"result_length", len(resultText),
		"duration_ms", duration.Milliseconds(),
		"model", model,
		"effort", effort,
		"is_deep", isDeepSearch,
	)

	return &WebSearchResponse{
		Result: resultText,
		Error:  "",
	}, nil
}

func (a *AgentSystem) ExtractPersonalization(
	log *tracing.Logger,
	currentProfile string,
	userMessage string,
) (*PersonalizationExtractionResponse, error) {
	defer tracing.ProfilePoint(log, "Agent extract personalization completed", "artificial.agents.extract.personalization", "message_length", len(userMessage))()
	a.metrics.RecordAgentUsage("personalization_extraction")

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), time.Duration(a.config.AI.Agents.PersonalizationExtractor.Timeout)*time.Second)
	defer cancel()

	prompt := a.getPersonalizationExtractionPrompt()
	systemMessage := fmt.Sprintf(prompt, currentProfile, userMessage)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: systemMessage},
		},
		{
			Role:    openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{Text: "Analyze the message and extract any new personal information. Return your response in JSON format."},
		},
	}

	model := a.config.AI.Agents.PersonalizationExtractor.Model

	request := openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
		Temperature: 0.1,
		Transforms:  []string{},
	}

	log = log.With("ai_agent", "personalization_extractor", tracing.AiModel, model)

	startTime := time.Now()
	response, err := a.ai.CreateChatCompletion(ctx, request)
	duration := time.Since(startTime)

	if err != nil {
		log.E("Failed to extract personalization", tracing.InnerError, err, "duration_ms", duration.Milliseconds())
		return nil, err
	}

	if len(response.Choices) == 0 {
		log.E("Empty choices in personalization extraction response")
		return nil, fmt.Errorf("empty response from personalization extraction agent")
	}

	responseText := cleanJSONFromMarkdown(response.Choices[0].Message.Content.Text)

	var extractionResponse PersonalizationExtractionResponse
	if err := json.Unmarshal([]byte(responseText), &extractionResponse); err != nil {
		log.E("Failed to parse personalization extraction response", tracing.InnerError, err, "response_text", responseText)
		return nil, fmt.Errorf("failed to parse extraction response: %w", err)
	}

	log.I("agent_personalization_extraction_success",
		"has_new_info", extractionResponse.HasNewInfo,
		"new_facts_count", len(extractionResponse.NewFacts),
		"duration_ms", duration.Milliseconds(),
	)

	return &extractionResponse, nil
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
