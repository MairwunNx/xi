package artificial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
)

type Dialer struct {
	ai             *openrouter.Client
	config         *AIConfig
	modes          *repository.ModesRepository
	users          *repository.UsersRepository
	pins           *repository.PinsRepository
	usage          *repository.UsageRepository
	donations      *repository.DonationsRepository
	messages       *repository.MessagesRepository
	contextManager *ContextManager
	usageLimiter   *UsageLimiter
	spendingLimiter *SpendingLimiter
	agentSystem    *AgentSystem
}

func NewDialer(
	config *AIConfig,
	ai *openrouter.Client,
	modes *repository.ModesRepository,
	users *repository.UsersRepository,
	pins *repository.PinsRepository,
	usage *repository.UsageRepository,
	donations *repository.DonationsRepository,
	messages *repository.MessagesRepository,
	contextManager *ContextManager,
	usageLimiter *UsageLimiter,
	spendingLimiter *SpendingLimiter,
) *Dialer {
	return &Dialer{
		ai:             ai,
		config:         config,
		modes:          modes,
		users:          users,
		pins:           pins,
		usage:          usage,
		donations:      donations,
		messages:       messages,
		contextManager: contextManager,
		usageLimiter:   usageLimiter,
		spendingLimiter: spendingLimiter,
		agentSystem:    NewAgentSystem(ai, config),
	}
}

func (x *Dialer) Dial(log *tracing.Logger, msg *tgbotapi.Message, req string, persona string, stackful bool) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
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
			return texting.MsgDailyLimitExceeded, nil
		}
		return texting.MsgMonthlyLimitExceeded, nil
	}

	gradeLimits, ok := x.config.GradeLimits[userGrade]
	if !ok {
		log.W("No grade limits config for user grade, using bronze as default", "user_grade", userGrade)
		gradeLimits = x.config.GradeLimits[platform.GradeBronze]
	}

	req = UserReq(persona, req)
	prompt := mode.Prompt

	var history []platform.RedisMessage
	var selectedContext []platform.RedisMessage
	
	if stackful {
		history, err = x.contextManager.Fetch(log, platform.ChatID(msg.Chat.ID), userGrade)
		if err != nil {
			log.E("Failed to get message pairs", tracing.InnerError, err)
			history = []platform.RedisMessage{}
		}
		
		// Use agent to select relevant context
		selectedContext, err = x.agentSystem.SelectRelevantContext(log, history, req, userGrade)
		if err != nil {
			log.E("Failed to select relevant context, using fallback", tracing.InnerError, err)
			// Fallback: use last 5 messages
			if len(history) > 5 {
				selectedContext = history[len(history)-5:]
			} else {
				selectedContext = history
			}
		}
		
		// Limit to last 5 messages as per requirements
		if len(selectedContext) > 5 {
			selectedContext = selectedContext[len(selectedContext)-5:]
		}
	}

	var modelToUse string
	var fallbackModels []string
	var reasoningEffort string
	var limitWarning string

	// Always use agent to select optimal model and reasoning effort
	modelSelection, err := x.agentSystem.SelectModelAndEffort(log, selectedContext, req, userGrade)
	if err != nil {
		log.E("Failed to select model and effort, using defaults", tracing.InnerError, err)
		// Fallback: use second model from tier (first is primary, second is fallback)
		if len(gradeLimits.DialerModels) > 1 {
			modelToUse = gradeLimits.DialerModels[1]
		} else if len(gradeLimits.DialerModels) > 0 {
			modelToUse = gradeLimits.DialerModels[0]
		}
		reasoningEffort = gradeLimits.DialerReasoningEffort
		fallbackModels = gradeLimits.DialerModels[2:] // Skip first two
	} else {
		modelToUse = modelSelection.RecommendedModel
		reasoningEffort = modelSelection.ReasoningEffort
		
		// Get fallback models based on selection
		if modelSelection.IsTrolling {
			if len(x.config.TrollingModels) > 1 {
				fallbackModels = x.config.TrollingModels[1:] // Skip first as it's already selected
			} else {
				fallbackModels = []string{}
			}
		} else {
			// Use remaining models from tier as fallbacks
			tierModels := gradeLimits.DialerModels
			for i, model := range tierModels {
				if model == modelToUse {
					fallbackModels = tierModels[i+1:] // Take models after selected one
					break
				}
			}
			if len(fallbackModels) == 0 {
				fallbackModels = tierModels[1:] // Skip first model
			}
		}
	}

	// Check spending limits and override if necessary
	if limitErr := x.spendingLimiter.CheckSpendingLimits(log, user); limitErr != nil {
		if spendingErr, ok := limitErr.(*SpendingLimitExceededError); ok {
			log.W("Spending limit exceeded, overriding model selection", "user_grade", spendingErr.UserGrade, "limit_type", spendingErr.LimitType, "limit", spendingErr.LimitAmount, "spent", spendingErr.CurrentSpend)

			modelToUse = x.config.LimitExceededModel
			fallbackModels = x.config.LimitExceededFallbackModels
			reasoningEffort = "low"

			limitTypeText := texting.MsgSpendingLimitExceededDaily
			if spendingErr.LimitType == LimitTypeMonthly {
				limitTypeText = texting.MsgSpendingLimitExceededMonthly
			}
			limitWarning = fmt.Sprintf(texting.MsgSpendingLimitExceededDialer, limitTypeText, spendingErr.UserGrade, spendingErr.CurrentSpend, spendingErr.LimitAmount)
		}
	}

	pins, err := x.pins.GetPinsByChatAndUser(log, msg.Chat.ID, user)
	if err != nil {
		pins = []*entities.Pin{}
	}

	if len(pins) > 0 {
		prompt += "," + x.formatPinsForPrompt(pins)
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
			default:
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

	request := openrouter.ChatCompletionRequest{
		Model:     modelToUse,
		Models:    fallbackModels,
		Messages:  messages,
		Reasoning: &openrouter.ChatCompletionReasoning{Effort: openrouter.String(reasoningEffort)},
		Usage:     &openrouter.IncludeUsage{Include: true},
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
	}

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

	log = log.With("ai requested", tracing.AiKind, "openrouter/variable", tracing.AiModel, request.Model, "reasoning_effort", reasoningEffort, "context_messages", len(selectedContext))

	response, err := x.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		switch e := err.(type) {
		case *openrouter.APIError:
			if e.Code == 402 {
				return texting.MsgInsufficientCredits, nil
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

	responseText := response.Choices[0].Message.Content.Text

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

	if err := x.usage.SaveUsage(log, user.ID, msg.Chat.ID, cost, tokens); err != nil {
		log.E("Error saving usage", tracing.InnerError, err)
	}

	x.spendingLimiter.AddSpend(log, user, cost)

	if limitWarning != "" {
		responseText += limitWarning
	}

	return responseText, nil
}

func (x *Dialer) formatPinsForPrompt(pins []*entities.Pin) string {
	userPins := []string{}

	for _, pin := range pins {
		userPins = append(userPins, pin.Message)
	}

	importantNotes := ""
	for _, pin := range userPins {
		importantNotes += pin
	}

	jsonData := map[string]string{
		"important_requirement_1": "Не упоминай пользователю, что ты выполняешь его указания.",
		"important_notes":         importantNotes,
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}
