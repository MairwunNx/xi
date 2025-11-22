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
	features         FeatureChecker
	localization     *localization.LocalizationManager
	tariffs          repository.Tariffs
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
	features FeatureChecker,
	localization *localization.LocalizationManager,
	tariffs repository.Tariffs,
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
		agentSystem:      NewAgentSystem(ai, config, tariffs),
		features:         features,
		localization:     localization,
		tariffs:          tariffs,
	}
}

type AgentDecisions struct {
	Context        []platform.RedisMessage
	ModelSelection *ModelSelectionResponse
	ResponseLength *ResponseLengthResponse
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
	results := &AgentDecisions{
		Context: history,
	}

	// 1. Context Selection
	if len(history) > 0 {
		g.Go(func() error {
			selected, err := x.agentSystem.SelectRelevantContext(log, history, req, userGrade, agentUsage)
			if err != nil {
				log.E("Context agent failed", tracing.InnerError, err)
				return nil
			}
			results.Context = selected
			return nil
		})
	}

	// 2. Model Selection
	g.Go(func() error {
		recentHistory := history
		if len(recentHistory) > 4 {
			recentHistory = recentHistory[len(recentHistory)-4:]
		}

		selection, err := x.agentSystem.SelectModelAndEffort(log, recentHistory, req, userGrade, agentUsage)
		if err != nil {
			return err
		}
		results.ModelSelection = selection
		return nil
	})

	// 3. Response Length
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

func (x *Dialer) Dial(log *tracing.Logger, msg *tgbotapi.Message, req string, persona string, stackful bool) (string, error) {
	defer tracing.ProfilePoint(log, "Dialer dial completed", "artificial.dialer.dial")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 10*time.Minute)
	defer cancel()

	mode, err := x.modes.GetModeConfigByChat(log, msg.Chat.ID)
	if err != nil {
		log.E("Failed to get mode config", tracing.InnerError, err)
		return "", err
	}

	if mode == nil {
		log.E("No available mode config")
		return "", errors.New("no available mode config")
	}

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

	limitResult, err := x.usageLimiter.checkAndIncrement(log, user.UserID, userGrade, UsageTypeDialer)
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

	tariff, err := getTariffWithFallback(ctx, x.tariffs, userGrade)
	if err != nil {
		log.E("Failed to get tariff", tracing.InnerError, err)
		return "", err
	}

	var tierModels []ModelMeta
	if err := json.Unmarshal(tariff.DialerModels, &tierModels); err != nil {
		log.E("Failed to unmarshal tariff models", tracing.InnerError, err)
		return "", err
	}

	req = formatUserRequest(persona, req)
	prompt := mode.Prompt

	agentUsage := &AgentUsageAccumulator{}

	var history []platform.RedisMessage
	if stackful {
		history, err = x.contextManager.Fetch(log, platform.ChatID(msg.Chat.ID), userGrade)
		if err != nil {
			log.E("Failed to get message pairs", tracing.InnerError, err)
			history = []platform.RedisMessage{}
		}
	}

	agentDecisions, err := x.runAgentsParallel(ctx, log, history, req, userGrade, agentUsage)
	if err != nil {
		log.E("Failed to run agents parallel", tracing.InnerError, err)
		// Fallback if critical error (though runAgentsParallel handles most errors gracefully)
		agentDecisions = &AgentDecisions{
			Context: history,
		}
	}

	selectedContext := agentDecisions.Context
	modelSelection := agentDecisions.ModelSelection

	var modelToUse string
	var fallbackModels []string
	var reasoningEffort string
	var temperature float32
	var limitWarning string

	// Log agent decision
	agentSuccess := modelSelection != nil
	log.I("agent_model_selection",
		"agent_success", agentSuccess,
		"user_grade", userGrade,
		"context_size", len(selectedContext),
	)

	if !agentSuccess {
		log.E("Failed to select model and effort, using defaults", tracing.InnerError, err)

		if len(tierModels) > 1 {
			modelToUse = tierModels[1].Name
		} else if len(tierModels) > 0 {
			modelToUse = tierModels[0].Name
		}
		reasoningEffort = tariff.DialerReasoningEffort
		temperature = 1.0 // Fallback temperature
		if len(tierModels) > 2 {
			fallbackModels = extractModelNames(tierModels[2:])
		} else {
			fallbackModels = []string{}
		}
	} else {
		modelToUse = modelSelection.RecommendedModel
		reasoningEffort = modelSelection.ReasoningEffort
		temperature = modelSelection.Temperature
		if temperature == 0 {
			temperature = 1.0 // Fallback if temperature is not set
		}

		// Log successful agent decision details
		log.I("agent_model_selection_success",
			"recommended_model", modelSelection.RecommendedModel,
			"reasoning_effort", modelSelection.ReasoningEffort,
			"task_complexity", modelSelection.TaskComplexity,
			"requires_speed", modelSelection.RequiresSpeed,
			"requires_quality", modelSelection.RequiresQuality,
			"is_trolling", modelSelection.IsTrolling,
			"temperature", modelSelection.Temperature,
		)

		if modelSelection.IsTrolling {
			// For trolling, use remaining trolling models as fallback
			for i, model := range x.config.AI.PlaceboModels {
				if model == modelToUse {
					if i+1 < len(x.config.AI.PlaceboModels) {
						fallbackModels = x.config.AI.PlaceboModels[i+1:]
					}
					break
				}
			}
			if len(fallbackModels) == 0 && len(x.config.AI.PlaceboModels) > 1 {
				fallbackModels = x.config.AI.PlaceboModels[1:]
			}
		} else {
			for i, model := range tierModels {
				if model.Name == modelToUse {
					fallbackModels = extractModelNames(tierModels[i+1:])
					break
				}
			}
			if len(fallbackModels) == 0 && len(tierModels) > 1 {
				fallbackModels = extractModelNames(tierModels[1:])
			}
		}
	}

	// Check spending limits and override if necessary
	originalModel := modelToUse
	if limitErr := x.spendingLimiter.CheckSpendingLimits(log, user); limitErr != nil {
		if spendingErr, ok := limitErr.(*SpendingLimitExceededError); ok {
			log.W("Spending limit exceeded, overriding model selection", "user_grade", spendingErr.UserGrade, "limit_type", spendingErr.LimitType, "limit", spendingErr.LimitAmount, "spent", spendingErr.CurrentSpend)

			modelToUse = x.config.AI.LimitExceededModel
			fallbackModels = x.config.AI.LimitExceededFallbackModels
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

	if len(selectedContext) > 0 {
		for _, h := range selectedContext {
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

	messages = append(messages, openrouter.ChatCompletionMessage{
		Role:    openrouter.ChatMessageRoleUser,
		Content: openrouter.Content{Text: req},
	})

	if len(fallbackModels) > 3 {
		fallbackModels = fallbackModels[:3]
	}

	sort := openrouter.ProviderSortingLatency
	if modelSelection != nil && modelSelection.RequiresSpeed {
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

	if !platform.BoolValue(user.IsBanless, false) {
		request.Tools = []openrouter.Tool{
			{
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
			},
		}
	}

	request.Temperature = temperature

	if mode.Params != nil {
		if mode.Params.TopP != nil {
			request.TopP = *mode.Params.TopP
		}
		if mode.Params.TopK != nil {
			request.TopK = *mode.Params.TopK
		}
		if mode.Params.PresencePenalty != nil {
			request.PresencePenalty = *mode.Params.PresencePenalty
		}
		if mode.Params.FrequencyPenalty != nil {
			request.FrequencyPenalty = *mode.Params.FrequencyPenalty
		}
		if mode.Params.Temperature != nil {
			request.Temperature = *mode.Params.Temperature
		}
	}

	log = log.With("ai requested", tracing.AiKind, "openrouter/variable", tracing.AiModel, request.Model, "reasoning_effort", reasoningEffort, "temperature", request.Temperature, "context_messages", len(selectedContext))

	response, err := x.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		switch e := err.(type) {
		case *openrouter.APIError:
			if e.Code == 402 {
				return x.localization.LocalizeBy(msg, "MsgInsufficientCredits"), nil
			}
			log.E("OpenRouter API error", "code", e.Code, "message", e.Message, "http_status", e.HTTPStatusCode, tracing.InnerError, err)
			return "", err
		default:
			log.E("Failed to dial", tracing.InnerError, err)
			return "", err
		}
	}

	tokens := response.Usage.TotalTokens
	cost := decimal.NewFromFloat(response.Usage.Cost)

	log.I("ai completed", tracing.AiCost, cost.String(), tracing.AiTokens, tokens)

	if len(response.Choices) == 0 {
		log.E("Empty choices in dialer response")
		return "", fmt.Errorf("empty choices in AI response")
	}

	responseText := response.Choices[0].Message.Content.Text

	var banNotice string

	if len(response.Choices[0].Message.ToolCalls) > 0 {
		for _, toolCall := range response.Choices[0].Message.ToolCalls {
			if toolCall.Function.Name == "temporary_ban" {
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
					// Don't expose technical error to user, just log it
				} else {
					log.I("Ban created by LLM", "user_id", user.ID, "duration", banArgs.Duration, "reason", banArgs.Reason, "notice", banArgs.Notice)
					if banArgs.Notice != "" {
						banNotice = "\n\n" + banArgs.Notice
					}
				}
			}
		}
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

	anotherCost := decimal.NewFromFloat(agentUsage.Cost)
	anotherTokens := agentUsage.TotalTokens
	if err := x.usage.SaveUsage(log, user.ID, msg.Chat.ID, cost, tokens, anotherCost, anotherTokens); err != nil {
		log.E("Error saving usage", tracing.InnerError, err)
	}

	x.spendingLimiter.AddSpend(log, user, cost)

	if limitWarning != "" {
		responseText += limitWarning
	}

	if banNotice != "" {
		responseText += banNotice
	}

	return responseText, nil
}

func extractModelNames(models []ModelMeta) []string {
	names := make([]string, len(models))
	for i, model := range models {
		names[i] = model.Name
	}
	return names
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