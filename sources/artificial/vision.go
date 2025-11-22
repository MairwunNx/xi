package artificial

import (
	"context"
	"fmt"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/localization"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"ximanager/sources/persistence/entities"
	"ximanager/sources/repository"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
)

type Vision struct {
	ai              *openrouter.Client
	config          *configuration.Config
	usage           *repository.UsageRepository
	usageLimiter    *UsageLimiter
	users           *repository.UsersRepository
	donations       *repository.DonationsRepository
	spendingLimiter *SpendingLimiter
	localization    *localization.LocalizationManager
	tariffs         repository.Tariffs
}

func NewVision(
	config *configuration.Config,
	ai *openrouter.Client,
	usage *repository.UsageRepository,
	usageLimiter *UsageLimiter,
	users *repository.UsersRepository,
	donations *repository.DonationsRepository,
	spendingLimiter *SpendingLimiter,
	localization *localization.LocalizationManager,
	tariffs repository.Tariffs,
) *Vision {
	return &Vision{
		ai:              ai,
		config:          config,
		usage:           usage,
		usageLimiter:    usageLimiter,
		users:           users,
		donations:       donations,
		spendingLimiter: spendingLimiter,
		localization:    localization,
		tariffs:         tariffs,
	}
}

func (v *Vision) Visionify(logger *tracing.Logger, msg *tgbotapi.Message, iurl string, user *entities.User, chatID int64, req string, persona string) (string, error) {
	defer tracing.ProfilePoint(logger, "Vision visionify completed", "artificial.vision.visionify")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	userGrade, err := v.donations.GetUserGrade(logger, user)
	if err != nil {
		logger.W("Failed to get user grade, using bronze as default", tracing.InnerError, err)
		userGrade = platform.GradeBronze
	}

	limitResult, err := v.usageLimiter.checkAndIncrement(logger, user.UserID, userGrade, UsageTypeVision)
	if err != nil {
		logger.E("Failed to check usage limits", tracing.InnerError, err)
		return "", err
	}

	if limitResult.Exceeded {
		if limitResult.IsDaily {
			return v.localization.LocalizeBy(msg, "MsgDailyLimitExceeded"), nil
		}
		return v.localization.LocalizeBy(msg, "MsgMonthlyLimitExceeded"), nil
	}

	tariff, err := getTariffWithFallback(ctx, v.tariffs, userGrade)
	if err != nil {
		logger.E("Failed to get tariff", tracing.InnerError, err)
		return "", err
	}

	modelToUse := tariff.VisionPrimaryModel
	fallbackModels := tariff.VisionFallbackModels

	var limitWarning string
	modelDowngraded := false
	originalModel := modelToUse

	if limitErr := v.spendingLimiter.CheckSpendingLimits(logger, user); limitErr != nil {
		if spendingErr, ok := limitErr.(*SpendingLimitExceededError); ok {
			logger.W("Spending limit exceeded for vision, using fallback model", "user_grade", spendingErr.UserGrade, "limit_type", spendingErr.LimitType, "limit", spendingErr.LimitAmount, "spent", spendingErr.CurrentSpend)

			modelToUse = v.config.AI.LimitExceededModel
			fallbackModels = v.config.AI.LimitExceededFallbackModels
			modelDowngraded = true

			periodText := v.localization.LocalizeBy(msg, "MsgSpendingLimitExceededDaily")
			if spendingErr.LimitType == LimitTypeMonthly {
				periodText = v.localization.LocalizeBy(msg, "MsgSpendingLimitExceededMonthly")
			}

			limitWarning = v.localization.LocalizeByTd(msg, "MsgSpendingLimitExceededVision", map[string]interface{}{
				"Period": periodText,
				"Grade":  spendingErr.UserGrade,
				"Spent":  spendingErr.CurrentSpend.String(),
				"Limit":  spendingErr.LimitAmount.String(),
			})
		}
	}

	logger.I("vision_model_selection",
		"user_grade", userGrade,
		"selected_model", modelToUse,
		"original_model", originalModel,
		"model_downgraded", modelDowngraded,
		"fallback_models_count", len(fallbackModels),
	)

	if req == "" {
		req = v.localization.LocalizeBy(msg, "InternalAIImageMessage")
	}

	req = formatUserRequest(persona, req)

	messages := []openrouter.ChatCompletionMessage{
		{
			Role: openrouter.ChatMessageRoleUser,
			Content: openrouter.Content{
				Multi: []openrouter.ChatMessagePart{
					{Type: openrouter.ChatMessagePartTypeText, Text: req},
					{Type: openrouter.ChatMessagePartTypeImageURL, ImageURL: &openrouter.ChatMessageImageURL{URL: iurl, Detail: openrouter.ImageURLDetailHigh}},
				},
			},
		},
	}

	request := openrouter.ChatCompletionRequest{
		Model:    modelToUse,
		Models:   fallbackModels,
		Messages: messages,
		Usage:    &openrouter.IncludeUsage{Include: true},
		Provider: &openrouter.ChatProvider{
			DataCollection: openrouter.DataCollectionDeny,
			Sort:           openrouter.ProviderSortingLatency,
		},
	}

	logger = logger.With(tracing.AiKind, "openrouter/vision", tracing.AiModel, request.Model)

	logger.I("vision requested")

	response, err := v.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		switch e := err.(type) {
		case *openrouter.APIError:
			if e.Code == 402 {
				return v.localization.LocalizeBy(msg, "MsgInsufficientCredits"), nil
			}
			logger.E("OpenRouter API error in vision", "code", e.Code, "message", e.Message, "http_status", e.HTTPStatusCode, tracing.InnerError, err)
			return "", err
		default:
			logger.E("Failed to visionify image", tracing.InnerError, err)
			return "", err
		}
	}

	agentUsage := &AgentUsageAccumulator{}

	tokens := response.Usage.TotalTokens
	cost := decimal.NewFromFloat(response.Usage.Cost)

	anotherCost := decimal.NewFromFloat(agentUsage.Cost)
	anotherTokens := agentUsage.TotalTokens

	if err := v.usage.SaveUsage(logger, user.ID, chatID, cost, tokens, anotherCost, anotherTokens); err != nil {
		logger.E("Error saving usage", tracing.InnerError, err)
	}

	v.spendingLimiter.AddSpend(logger, user, cost)

	if len(response.Choices) == 0 {
		logger.E("Empty choices in vision response")
		return "", fmt.Errorf("empty choices in AI response")
	}

	responseText := response.Choices[0].Message.Content.Text

	if limitWarning != "" {
		responseText += limitWarning
	}

	logger.I("vision completed", tracing.AiCost, cost.String(), tracing.AiTokens, tokens)

	return responseText, nil
}