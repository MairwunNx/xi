package artificial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/features"
	"ximanager/sources/localization"
	"ximanager/sources/metrics"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

type Dialer struct {
	ai               *openrouter.Client
	config           *configuration.Config
	modes            *repository.ModesRepository
	users            *repository.UsersRepository
	personalizations *repository.PersonalizationsRepository
	usage            *repository.UsageRepository
	donations        *repository.DonationsRepository
	messages         *repository.MessagesRepository
	bans             *repository.BansRepository
	contextManager   *ContextManager
	usageLimiter     *UsageLimiter
	spendingLimiter  *SpendingLimiter
	agentSystem      *AgentSystem
	features         *features.FeatureManager
	localization     *localization.LocalizationManager
	tariffs          *repository.TariffsRepository
	metrics          *metrics.MetricsService
	log              *tracing.Logger
}

func NewDialer(
	config *configuration.Config,
	ai *openrouter.Client,
	modes *repository.ModesRepository,
	users *repository.UsersRepository,
	personalizations *repository.PersonalizationsRepository,
	usage *repository.UsageRepository,
	donations *repository.DonationsRepository,
	messages *repository.MessagesRepository,
	bans *repository.BansRepository,
	contextManager *ContextManager,
	usageLimiter *UsageLimiter,
	spendingLimiter *SpendingLimiter,
	fm *features.FeatureManager,
	localization *localization.LocalizationManager,
	tariffs *repository.TariffsRepository,
	metrics *metrics.MetricsService,
	log *tracing.Logger,
) *Dialer {
	return &Dialer{
		ai:               ai,
		config:           config,
		modes:            modes,
		users:            users,
		personalizations: personalizations,
		usage:            usage,
		donations:        donations,
		messages:         messages,
		bans:             bans,
		contextManager:   contextManager,
		usageLimiter:     usageLimiter,
		spendingLimiter:  spendingLimiter,
		agentSystem:      NewAgentSystem(ai, config, tariffs, metrics, log),
		features:         fm,
		localization:     localization,
		tariffs:          tariffs,
		metrics:          metrics,
		log:              log,
	}
}

type AgentDecisions struct {
	EffortSelection *EffortSelectionResponse
	ResponseLength  *ResponseLengthResponse
}

func (x *Dialer) runAgentsParallel(
	ctx context.Context,
	log *tracing.Logger,
	history []platform.RedisMessage,
	req string,
	userGrade platform.UserGrade,
	agentUsage *AgentUsageAccumulator,
) (*AgentDecisions, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := &AgentDecisions{}

	// 1. Effort Selection
	g.Go(func() error {
		recentHistory := history
		if len(recentHistory) > 6 {
			recentHistory = recentHistory[len(recentHistory)-6:]
		}

		selection, err := x.agentSystem.SelectEffort(log, recentHistory, req, userGrade, agentUsage)
		if err != nil {
			return err
		}
		results.EffortSelection = selection
		return nil
	})

	// 2. Response Length
	if x.features.IsEnabled(features.FeatureResponseLengthDetection) {
		g.Go(func() error {
			length, err := x.agentSystem.DetermineResponseLength(log, req, agentUsage)
			if err != nil {
				log.W("Length agent failed", tracing.InnerError, err)
				return nil
			}
			results.ResponseLength = length
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

func (x *Dialer) Dial(log *tracing.Logger, msg *tgbotapi.Message, req string, imageURL string, persona string, stackful bool) (string, error) {
	defer tracing.ProfilePoint(log, "Dialer dial completed", "artificial.dialer.dial")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 10*time.Minute)
	defer cancel()

	mode, err := x.modes.GetCurrentModeForChat(log, msg.Chat.ID)
	if err != nil {
		log.E("Failed to get mode config", tracing.InnerError, err)
		return "", err
	}

	if mode == nil {
		log.E("No available mode config")
		return "", errors.New("no available mode config")
	}

	modeConfig := x.modes.ParseModeConfig(mode, log)

	user, err := x.users.GetUserByEid(log, msg.From.ID)
	if err != nil {
		log.E("Failed to get user", tracing.InnerError, err)
		return "", err
	}

	userGrade, err := x.donations.GetUserGrade(log, user)
	if err != nil {
		log.W("Failed to get user grade, using bronze as default", tracing.InnerError, err)
		userGrade = platform.GradeBronze
	}

	usageType := UsageTypeDialer
	if imageURL != "" {
		usageType = UsageTypeVision
	}

	limitResult, err := x.usageLimiter.checkAndIncrement(log, user.UserID, userGrade, usageType)
	if err != nil {
		log.E("Failed to check usage limits", tracing.InnerError, err)
		return "", err
	}

	if limitResult.Exceeded {
		if limitResult.IsDaily {
			return x.localization.LocalizeBy(msg, "MsgDailyLimitExceeded"), nil
		}
		return x.localization.LocalizeBy(msg, "MsgMonthlyLimitExceeded"), nil
	}

	tokenLimitResult, err := x.usageLimiter.CheckTokenLimits(log, user.UserID, userGrade)
	if err != nil {
		log.E("Failed to check token limits", tracing.InnerError, err)
		return "", err
	}

	if tokenLimitResult.Exceeded {
		if tokenLimitResult.IsDaily {
			return x.localization.LocalizeBy(msg, "MsgDailyTokenLimitExceeded"), nil
		}
		return x.localization.LocalizeBy(msg, "MsgMonthlyTokenLimitExceeded"), nil
	}

	var tariffModelConfig configuration.AI_TariffModelConfig
	switch userGrade {
	case platform.GradeBronze:
		tariffModelConfig = x.config.AI.TariffModels.Bronze
	case platform.GradeSilver:
		tariffModelConfig = x.config.AI.TariffModels.Silver
	case platform.GradeGold:
		tariffModelConfig = x.config.AI.TariffModels.Gold
	default:
		tariffModelConfig = x.config.AI.TariffModels.Bronze
	}

	req = formatUserRequest(persona, req)
	prompt := modeConfig.Prompt

	agentUsage := &AgentUsageAccumulator{}

	var history []platform.RedisMessage
	summarizationOccurred := false
	if stackful {
		history, summarizationOccurred, err = x.contextManager.Fetch(log, platform.ChatID(msg.Chat.ID), userGrade)
		if err != nil {
			log.E("Failed to get message pairs", tracing.InnerError, err)
			history = []platform.RedisMessage{}
		}
	}

	agentDecisions, err := x.runAgentsParallel(ctx, log, history, req, userGrade, agentUsage)
	if err != nil {
		log.E("Failed to run agents parallel", tracing.InnerError, err)
		agentDecisions = &AgentDecisions{}
	}

	effortSelection := agentDecisions.EffortSelection

	modelToUse := tariffModelConfig.PrimaryModel
	fallbackModel := tariffModelConfig.FallbackModel
	var reasoningEffort string
	var temperature float32
	var limitWarning string

	agentSuccess := effortSelection != nil
	log.I("agent_effort_selection",
		"agent_success", agentSuccess,
		"user_grade", userGrade,
		"context_size", len(history),
	)

	if !agentSuccess {
		log.E("Effort selection agent failed or returned nil, using defaults")
		reasoningEffort = "medium"
		temperature = 1.0
	} else {
		reasoningEffort = effortSelection.ReasoningEffort

		if modeConfig.Params != nil && modeConfig.Params.Temperature != nil && modeConfig.Final {
			if *modeConfig.Params.Temperature == 0 {
				temperature = 1.0
			} else {
				temperature = *modeConfig.Params.Temperature
			}
		} else {
			temperature = effortSelection.Temperature
			if temperature == 0 {
				temperature = 1.0
			}
		}

		log.I("agent_effort_selection_success",
			"reasoning_effort", effortSelection.ReasoningEffort,
			"task_complexity", effortSelection.TaskComplexity,
			"requires_speed", effortSelection.RequiresSpeed,
			"requires_quality", effortSelection.RequiresQuality,
			"temperature", effortSelection.Temperature,
		)
	}

	originalModel := modelToUse
	if limitErr := x.spendingLimiter.CheckSpendingLimits(log, user); limitErr != nil {
		if spendingErr, ok := limitErr.(*SpendingLimitExceededError); ok {
			log.W("Spending limit exceeded, overriding model selection", "user_grade", spendingErr.UserGrade, "limit_type", spendingErr.LimitType, "limit", spendingErr.LimitAmount, "spent", spendingErr.CurrentSpend)

			modelToUse = x.config.AI.LimitExceededModel
			fallbackModel = ""
			if len(x.config.AI.LimitExceededFallbackModels) > 0 {
				fallbackModel = x.config.AI.LimitExceededFallbackModels[0]
			}
			reasoningEffort = "low"

			log.I("spending_limit_override",
				"original_model", originalModel,
				"override_model", modelToUse,
				"limit_type", spendingErr.LimitType,
				"user_grade", spendingErr.UserGrade,
				"current_spend", spendingErr.CurrentSpend.String(),
				"limit_amount", spendingErr.LimitAmount.String(),
			)

			periodText := x.localization.LocalizeBy(msg, "MsgSpendingLimitExceededDaily")
			if spendingErr.LimitType == LimitTypeMonthly {
				periodText = x.localization.LocalizeBy(msg, "MsgSpendingLimitExceededMonthly")
			}

			limitWarning = x.localization.LocalizeByTd(msg, "MsgSpendingLimitExceededDialer", map[string]interface{}{
				"Period": periodText,
				"Grade":  spendingErr.UserGrade,
				"Spent":  spendingErr.CurrentSpend.String(),
				"Limit":  spendingErr.LimitAmount.String(),
			})
		}
	}

	prompt += x.formatEnvironmentBlock(msg)

	personalization, err := x.personalizations.GetPersonalizationByUser(log, user)
	personalizationUsed := false
	if err == nil && personalization != nil {
		prompt += fmt.Sprintf(PersonalizationBlockTemplate, personalization.Prompt)
		personalizationUsed = true
	}

	log.I("dialer_personalization_status",
		"personalization_used", personalizationUsed,
		"user_id", user.ID,
	)

	if agentDecisions.ResponseLength != nil {
		log.I("response_length_detected",
			"length", agentDecisions.ResponseLength.Length,
			"confidence", agentDecisions.ResponseLength.Confidence,
			"reasoning", agentDecisions.ResponseLength.Reasoning,
		)
		if guideline := x.getResponseLengthGuideline(agentDecisions.ResponseLength.Length); guideline != "" {
			prompt += guideline
		}
	}

	messages := []openrouter.ChatCompletionMessage{
		{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: prompt},
		},
	}

	if len(history) > 0 {
		for _, h := range history {
			var role string
			switch h.Role {
			case platform.MessageRoleUser:
				role = openrouter.ChatMessageRoleUser
			case platform.MessageRoleAssistant:
				role = openrouter.ChatMessageRoleAssistant
			case platform.MessageRoleSystem:
				role = openrouter.ChatMessageRoleSystem
			default:
				log.W("Unknown message role in context, skipping", "role", h.Role)
				continue
			}
			messages = append(messages, openrouter.ChatCompletionMessage{
				Role:    role,
				Content: openrouter.Content{Text: h.Content},
			})
		}
	}

	userContent := openrouter.Content{
		Multi: []openrouter.ChatMessagePart{
			{Type: openrouter.ChatMessagePartTypeText, Text: req},
		},
	}

	if imageURL != "" {
		userContent.Multi = append(userContent.Multi, openrouter.ChatMessagePart{
			Type:     openrouter.ChatMessagePartTypeImageURL,
			ImageURL: &openrouter.ChatMessageImageURL{URL: imageURL, Detail: openrouter.ImageURLDetailHigh},
		})
	}

	messages = append(messages, openrouter.ChatCompletionMessage{
		Role:    openrouter.ChatMessageRoleUser,
		Content: userContent,
	})

	fallbackModels := []string{}
	if fallbackModel != "" {
		fallbackModels = []string{fallbackModel}
	}

	sort := openrouter.ProviderSortingLatency
	if effortSelection != nil && effortSelection.RequiresSpeed {
		sort = openrouter.ProviderSortingThroughput
	}

	request := openrouter.ChatCompletionRequest{
		Model:     modelToUse,
		Models:    fallbackModels,
		Messages:  messages,
		Reasoning: &openrouter.ChatCompletionReasoning{Effort: openrouter.String(reasoningEffort)},
		Usage:     &openrouter.IncludeUsage{Include: true},
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           sort,
		},
		User: strconv.FormatInt(msg.Chat.ID, 10) + "_" + user.ID.String(),
	}

	request.Transforms = []string{}

	request.Tools = x.buildTools(user)

	request.Temperature = temperature

	if modeConfig.Params != nil {
		if modeConfig.Params.TopP != nil {
			request.TopP = *modeConfig.Params.TopP
		}
		if modeConfig.Params.TopK != nil {
			request.TopK = *modeConfig.Params.TopK
		}
		if modeConfig.Params.PresencePenalty != nil {
			request.PresencePenalty = *modeConfig.Params.PresencePenalty
		}
		if modeConfig.Params.FrequencyPenalty != nil {
			request.FrequencyPenalty = *modeConfig.Params.FrequencyPenalty
		}
	}

	log = log.With("ai requested", tracing.AiKind, "openrouter/variable", tracing.AiModel, request.Model, "reasoning_effort", reasoningEffort, "temperature", request.Temperature, "context_messages", len(history))

	var responseText string
	var banNotice string
	var totalTokens int
	var totalCost decimal.Decimal
	var cacheReadTokens int
	var cacheWriteTokens int
	webSearchCalls := 0
	maxWebSearchCalls := x.config.AI.Agents.WebSearch.MaxCallsPerQuery
	if maxWebSearchCalls <= 0 {
		maxWebSearchCalls = 3
	}

	responseText, banNotice, totalTokens, totalCost, cacheReadTokens, cacheWriteTokens, err = x.dialNonStreaming(ctx, log, user, msg, request, messages, modelToUse, userGrade, agentUsage, &webSearchCalls, maxWebSearchCalls)
	if err != nil {
		return "", err
	}

	if err := x.messages.SaveMessage(log, msg, false); err != nil {
		log.E("Error saving user message", tracing.InnerError, err)
	}
	if err := x.messages.SaveMessage(log, msg, true); err != nil {
		log.E("Error saving Xi response", tracing.InnerError, err)
	}

	userMessage := platform.RedisMessage{Role: platform.MessageRoleUser, Content: req}
	if err := x.contextManager.Store(log, platform.ChatID(msg.Chat.ID), userGrade, userMessage); err != nil {
		log.E("Error saving user message to context", tracing.InnerError, err)
	}

	assistantMessage := platform.RedisMessage{Role: platform.MessageRoleAssistant, Content: responseText}
	if err := x.contextManager.Store(log, platform.ChatID(msg.Chat.ID), userGrade, assistantMessage); err != nil {
		log.E("Error saving assistant message to context", tracing.InnerError, err)
	}

	anotherCost := decimal.NewFromFloat(agentUsage.GetCost())
	anotherTokens := agentUsage.GetTotalTokens()
	if err := x.usage.SaveUsage(log, user.ID, msg.Chat.ID, totalCost, totalTokens, cacheReadTokens, cacheWriteTokens, anotherCost, anotherTokens); err != nil {
		log.E("Error saving usage", tracing.InnerError, err)
	}

	totalTokensUsed := totalTokens + anotherTokens
	if err := x.usageLimiter.AddTokens(log, user.UserID, totalTokensUsed); err != nil {
		log.E("Error adding tokens to limiter", tracing.InnerError, err)
	}

	x.metrics.RecordDialerUsage(totalTokens, totalCost.InexactFloat64(), modelToUse)
	if anotherTokens > 0 || !anotherCost.IsZero() {
		x.metrics.RecordAgentCost(anotherTokens, anotherCost.InexactFloat64(), modelToUse)
	}

	x.spendingLimiter.AddSpend(log, user, totalCost)

	if x.features.IsEnabled(features.FeaturePersonalizationExtraction) {
		go x.extractAndSavePersonalization(log, user, req, personalization)
	}

	if limitWarning != "" {
		responseText += limitWarning
	}

	if banNotice != "" {
		responseText += banNotice
	}

	if summarizationOccurred {
		summarizedNotice := x.localization.LocalizeBy(msg, "MsgChatSummarized")
		responseText = summarizedNotice + "\n\n" + responseText
	}

	return responseText, nil
}

func (x *Dialer) dialNonStreaming(
	ctx context.Context,
	log *tracing.Logger,
	user *entities.User,
	msg *tgbotapi.Message,
	request openrouter.ChatCompletionRequest,
	messages []openrouter.ChatCompletionMessage,
	modelToUse string,
	userGrade platform.UserGrade,
	agentUsage *AgentUsageAccumulator,
	webSearchCalls *int,
	maxWebSearchCalls int,
) (string, string, int, decimal.Decimal, int, int, error) {
	var responseText string
	var banNotice string
	var totalTokens int
	var totalCost decimal.Decimal
	var cacheReadTokens int
	var cacheWriteTokens int

	for {
		start := time.Now()
		response, err := x.ai.CreateChatCompletion(ctx, request)
		duration := time.Since(start)
		if err == nil {
			x.metrics.RecordAIRequestDuration(duration, modelToUse)
		}
		if err != nil {
			switch e := err.(type) {
			case *openrouter.APIError:
				if e.Code == 402 {
					return x.localization.LocalizeBy(msg, "MsgInsufficientCredits"), "", 0, decimal.Zero, 0, 0, nil
				}
				log.E("OpenRouter API error", "code", e.Code, "message", e.Message, "http_status", e.HTTPStatusCode, tracing.InnerError, err)
				return "", "", 0, decimal.Zero, 0, 0, err
			default:
				log.E("Failed to dial", tracing.InnerError, err)
				return "", "", 0, decimal.Zero, 0, 0, err
			}
		}

		totalTokens += response.Usage.TotalTokens
		totalCost = totalCost.Add(decimal.NewFromFloat(response.Usage.Cost))
		cacheReadTokens += response.Usage.PromptTokenDetails.CachedTokens

		log.I("ai iteration completed", tracing.AiCost, totalCost.String(), tracing.AiTokens, totalTokens, "iteration_tokens", response.Usage.TotalTokens)

		if len(response.Choices) == 0 {
			log.E("Empty choices in dialer response")
			return "", "", 0, decimal.Zero, 0, 0, fmt.Errorf("empty choices in AI response")
		}

	choice := response.Choices[0]
	responseText = choice.Message.Content.Text

	finishReason := choice.FinishReason
	log.I("ai_response_finish_reason", "finish_reason", finishReason)

	switch finishReason {
	case openrouter.FinishReasonLength:
		log.W("Response truncated due to length limit", "finish_reason", finishReason)
		lengthWarning := "\n\n" + x.localization.LocalizeBy(msg, "MsgFinishReasonLength")
		responseText += lengthWarning

	case openrouter.FinishReasonContentFilter:
		log.W("Content filtered by provider", "finish_reason", finishReason)
		filterNotice := "\n\n" + x.localization.LocalizeBy(msg, "MsgFinishReasonContentFilter")
		responseText += filterNotice

	case openrouter.FinishReasonStop:
		log.D("Normal completion", "finish_reason", finishReason)

	case openrouter.FinishReasonToolCalls:
		log.D("Response includes tool calls", "finish_reason", finishReason)

	case openrouter.FinishReasonNull:
		log.W("Finish reason is null", "finish_reason", finishReason)

	default:
		if finishReason != "" {
			log.W("Unknown finish reason", "finish_reason", finishReason)
		}
	}

	if len(choice.Message.ToolCalls) == 0 {
		break
	}

		toolResults := x.processToolCalls(log, user, msg, choice.Message.ToolCalls, &banNotice, webSearchCalls, maxWebSearchCalls, agentUsage)

		if len(toolResults) == 0 {
			break
		}

		messages = append(messages, openrouter.ChatCompletionMessage{
			Role:      openrouter.ChatMessageRoleAssistant,
			Content:   openrouter.Content{Text: responseText},
			ToolCalls: choice.Message.ToolCalls,
		})

		for _, toolResult := range toolResults {
			messages = append(messages, openrouter.ChatCompletionMessage{
				Role:       openrouter.ChatMessageRoleTool,
				Content:    openrouter.Content{Text: toolResult.Content},
				ToolCallID: toolResult.ToolCallID,
			})
		}

		request.Messages = messages
	}

	return responseText, banNotice, totalTokens, totalCost, cacheReadTokens, cacheWriteTokens, nil
}

type ToolResult struct {
	ToolCallID string
	Content    string
}

func (x *Dialer) processToolCalls(
	log *tracing.Logger,
	user *entities.User,
	msg *tgbotapi.Message,
	toolCalls []openrouter.ToolCall,
	banNotice *string,
	webSearchCalls *int,
	maxWebSearchCalls int,
	agentUsage *AgentUsageAccumulator,
) []ToolResult {
	var results []ToolResult

	for _, toolCall := range toolCalls {
		switch toolCall.Function.Name {
		case "web_search":
			if *webSearchCalls >= maxWebSearchCalls {
				log.W("Web search call limit reached", "max", maxWebSearchCalls)
				results = append(results, ToolResult{
					ToolCallID: toolCall.ID,
					Content:    "Web search limit reached for this query. Please provide your response based on the information already gathered.",
				})
				continue
			}
			*webSearchCalls++

			var searchArgs struct {
				Query        string `json:"query"`
				IsDeepSearch bool   `json:"is_deep_search"`
				Effort       string `json:"effort"`
			}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &searchArgs); err != nil {
				log.E("Failed to parse web_search tool arguments", tracing.InnerError, err)
				results = append(results, ToolResult{
					ToolCallID: toolCall.ID,
					Content:    "Error: Failed to parse search parameters.",
				})
				continue
			}

			log.I("Processing web_search tool call", "query", searchArgs.Query, "is_deep", searchArgs.IsDeepSearch, "effort", searchArgs.Effort, "call_number", *webSearchCalls)

			searchResult, err := x.agentSystem.WebSearch(log, searchArgs.Query, searchArgs.IsDeepSearch, searchArgs.Effort, agentUsage)
			if err != nil {
				log.E("Web search agent error", tracing.InnerError, err)
				results = append(results, ToolResult{
					ToolCallID: toolCall.ID,
					Content:    "Error: Web search failed. Please try to answer based on your knowledge.",
				})
				continue
			}

			if searchResult.Error != "" {
				results = append(results, ToolResult{
					ToolCallID: toolCall.ID,
					Content:    searchResult.Error,
				})
			} else {
				results = append(results, ToolResult{
					ToolCallID: toolCall.ID,
					Content:    searchResult.Result,
				})
			}

		case "temporary_ban":
			log.I("LLM called temporary_ban tool", "arguments", toolCall.Function.Arguments)

			var banArgs struct {
				Duration string `json:"duration"`
				Reason   string `json:"reason"`
				Notice   string `json:"notice"`
			}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &banArgs); err != nil {
				log.E("Failed to parse ban tool arguments", tracing.InnerError, err)
				continue
			}

			_, err := x.bans.CreateBan(log, user.ID, msg.Chat.ID, banArgs.Reason, banArgs.Duration)
			if err != nil {
				log.E("Failed to create ban from tool call", tracing.InnerError, err)
			} else {
				log.I("Ban created by LLM", "user_id", user.ID, "duration", banArgs.Duration, "reason", banArgs.Reason, "notice", banArgs.Notice)
				if banArgs.Notice != "" {
					*banNotice = "\n\n" + banArgs.Notice
				}
			}
		}
	}

	return results
}


func (x *Dialer) buildTools(user *entities.User) []openrouter.Tool {
	tools := []openrouter.Tool{
		{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "web_search",
				Description: "Search the web for current, real-time information. Use ONLY when:\n\n1. User explicitly asks about current events, news, or recent happenings\n2. Question involves time-sensitive data (prices, stocks, weather, sports scores)\n3. User asks 'what is happening now', 'latest news about', 'current status of'\n4. Need to verify facts that may have changed recently\n5. Question mentions specific dates in the future or recent past\n6. Looking for real-time statistics or live data\n\nDO NOT USE for:\n- General knowledge questions\n- Historical facts\n- Programming/coding help\n- Math calculations\n- Personal advice\n- Creative writing\n- Explaining concepts\n- Anything you already know with confidence",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query - be specific and include relevant keywords for better results",
						},
						"is_deep_search": map[string]interface{}{
							"type":        "boolean",
							"description": "Set true for complex research requiring in-depth analysis, multiple sources, or comprehensive overview. Set false for simple factual lookups or quick answers. Default: false",
						},
						"effort": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"low", "medium", "high"},
							"description": "Search effort level. 'low' for quick lookups, 'medium' for balanced search, 'high' for thorough research. Default: determined by config",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}

	if !platform.BoolValue(user.IsBanless, false) {
		tools = append(tools, openrouter.Tool{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        "temporary_ban",
				Description: "Temporarily ban user for violations. STRICT RULES:\n\nWHEN TO CALL:\n- Minimum 3 similar violations within last 10 messages\n- After explicit warning given (or include warning in current response)\n- Pattern of repeated behavior, NOT isolated incident\n- User ignored previous warning\n\nVIOLATION TYPES (severity → duration):\n1. Explicit prolonged rudeness/insults → 30m-2h\n2. Explicit prolonged trolling → 10m-1h\n3. Explicit prolonged spam/flood → 1m-10m\n4. Meaningless message chains → 1m-5m\n5. Very heavy computational tasks → 30s-2m\n\nDO NOT BAN FOR:\n- Criticism, disagreement, debate\n- Single off-topic messages\n- Poor language quality, typos, slang\n- Questions or confusion\n- First-time minor violations\n- Sarcasm or humor\n- Simple misunderstandings\n\nPROCESS:\n1. Warn user first (in current response)\n2. If violation continues → call this tool\n3. Tool will send notice to user automatically\n4. Do NOT mention ban in your response text\n\nMax ban: 12h. When in doubt, DON'T call.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"duration": map[string]interface{}{
							"type":        "string",
							"description": "Ban duration based on violation severity. Format: '30s', '1m', '5m', '10m', '30m', '1h', '2h', '4h', '12h'. Examples: heavy task=30s-1m, spam=1m-10m, rudeness=30m-2h",
							"enum":        []string{"30s", "45s", "1m", "90s", "2m", "3m", "5m", "7m", "10m", "15m", "20m", "30m", "45m", "1h", "90m", "2h", "3h", "4h", "5h", "6h", "8h", "10h", "12h"},
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "Brief internal reason tag (Russian). Examples: 'хамство', 'троллинг', 'флуд', 'спам', 'тяжелая задача'",
						},
						"notice": map[string]interface{}{
							"type":        "string",
							"description": "Full notice to user in Russian. Explain: what violated, why banned, duration, how to avoid future bans. Tone: firm but fair. Example: 'Временная блокировка на 10 минут за продолжительный флуд. Пожалуйста, избегайте отправки множества коротких бессмысленных сообщений подряд.'",
						},
					},
					"required": []string{"duration", "reason", "notice"},
				},
			},
		})
	}

	return tools
}

func (x *Dialer) getResponseLengthGuideline(length string) string {
	switch length {
	case "very_brief":
		return "\n\n### Response Length Guideline\n\n**Very brief response required** (1-2 sentences maximum):\n- Answer directly and concisely\n- One key fact or yes/no\n- No elaboration unless critical\n- Skip examples and details"
	case "brief":
		return "\n\n### Response Length Guideline\n\n**Brief response required** (3-5 sentences):\n- Short and focused explanation\n- Core information only\n- Minimal examples if needed\n- Skip tangential details"
	case "medium":
		return "\n\n### Response Length Guideline\n\n**Standard response** (balanced length):\n- Provide a complete, well-structured answer\n- Include relevant context and examples\n- Balance thoroughness with brevity\n- Natural conversational length"
	case "detailed":
		return "\n\n### Response Length Guideline\n\n**Detailed response required**:\n- Comprehensive explanation with context\n- Include multiple examples and perspectives\n- Cover edge cases and nuances\n- Thorough but organized presentation"
	case "very_detailed":
		return "\n\n### Response Length Guideline\n\n**Very detailed response required** (in-depth analysis):\n- Exhaustive coverage of the topic\n- Multiple examples, comparisons, and perspectives\n- Historical context and implications\n- Step-by-step breakdowns where applicable\n- All relevant details and edge cases"
	default:
		return "" // No guideline for unknown length
	}
}

func (x *Dialer) formatEnvironmentBlock(msg *tgbotapi.Message) string {
	localNow := time.Now().In(time.Local)
	dateTimeStr := localNow.Format("Monday, January 2, 2006 at 15:04")

	chatTitle := msg.Chat.Title
	if chatTitle == "" {
		if msg.Chat.Type == "private" {
			chatTitle = "Private chat"
		} else {
			chatTitle = "Untitled chat"
		}
	}

	chatDescription := msg.Chat.Description
	if chatDescription == "" {
		chatDescription = "No description"
	}

	version := platform.GetAppVersion()
	uptime := time.Since(platform.GetAppStartTime()).Truncate(time.Second)

	return fmt.Sprintf(EnvironmentBlockTemplate,
		dateTimeStr,
		chatTitle,
		chatDescription,
		version,
		uptime.String(),
	)
}

func (x *Dialer) extractAndSavePersonalization(log *tracing.Logger, user *entities.User, userMessage string, currentPersonalization *entities.Personalization) {
	defer tracing.ProfilePoint(log, "Background personalization extraction completed", "dialer.personalization.extraction")()

	currentProfile := ""
	if currentPersonalization != nil {
		currentProfile = currentPersonalization.Prompt
	}

	extraction, err := x.agentSystem.ExtractPersonalization(log, currentProfile, userMessage)
	if err != nil {
		log.W("Failed to extract personalization", tracing.InnerError, err)
		x.metrics.RecordPersonalizationExtracted("error")
		return
	}

	if !extraction.HasNewInfo || extraction.UpdatedProfile == nil || *extraction.UpdatedProfile == "" {
		log.D("No new personalization info found")
		x.metrics.RecordPersonalizationExtracted("no_new_info")
		return
	}

	_, err = x.personalizations.CreateOrUpdatePersonalization(log, user, *extraction.UpdatedProfile)
	if err != nil {
		log.E("Failed to save extracted personalization", tracing.InnerError, err)
		x.metrics.RecordPersonalizationExtracted("save_error")
		return
	}

	log.I("personalization_extracted_and_saved",
		"new_facts_count", len(extraction.NewFacts),
		"new_facts", extraction.NewFacts,
	)
	x.metrics.RecordPersonalizationExtracted("success")
}
