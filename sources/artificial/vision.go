package artificial

import (
	"context"
	"fmt"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	"ximanager/sources/persistence/entities"
	"ximanager/sources/repository"

	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
)

type Vision struct {
	ai              *openrouter.Client
	config          *AIConfig
	usage           *repository.UsageRepository
	usageLimiter    *UsageLimiter
	users           *repository.UsersRepository
	donations       *repository.DonationsRepository
	spendingLimiter *SpendingLimiter
}

func NewVision(
	config *AIConfig,
	ai *openrouter.Client,
	usage *repository.UsageRepository,
	usageLimiter *UsageLimiter,
	users *repository.UsersRepository,
	donations *repository.DonationsRepository,
	spendingLimiter *SpendingLimiter,
) *Vision {
	return &Vision{
		ai:              ai,
		config:          config,
		usage:           usage,
		usageLimiter:    usageLimiter,
		users:           users,
		donations:       donations,
		spendingLimiter: spendingLimiter,
	}
}

func (v *Vision) Visionify(logger *tracing.Logger, iurl string, user *entities.User, chatID int64, req string, persona string) (string, error) {
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
			return texting.MsgDailyLimitExceeded, nil
		}
		return texting.MsgMonthlyLimitExceeded, nil
	}

	gradeLimits, ok := v.config.GradeLimits[userGrade]
	if !ok {
		logger.W("No grade limits config for user grade, using bronze as default", "user_grade", userGrade)
		gradeLimits = v.config.GradeLimits[platform.GradeBronze]
	}

	modelToUse := gradeLimits.VisionPrimaryModel
	fallbackModels := gradeLimits.VisionFallbackModels
	var limitWarning string
	modelDowngraded := false
	originalModel := modelToUse

	if limitErr := v.spendingLimiter.CheckSpendingLimits(logger, user); limitErr != nil {
		if spendingErr, ok := limitErr.(*SpendingLimitExceededError); ok {
			logger.W("Spending limit exceeded for vision, using fallback model", "user_grade", spendingErr.UserGrade, "limit_type", spendingErr.LimitType, "limit", spendingErr.LimitAmount, "spent", spendingErr.CurrentSpend)

			modelToUse = v.config.LimitExceededModel
			fallbackModels = v.config.LimitExceededFallbackModels
			modelDowngraded = true

			limitTypeText := texting.MsgSpendingLimitExceededDaily
			if spendingErr.LimitType == LimitTypeMonthly {
				limitTypeText = texting.MsgSpendingLimitExceededMonthly
			}
			limitWarning = fmt.Sprintf(texting.MsgSpendingLimitExceededVision, limitTypeText, spendingErr.UserGrade, spendingErr.CurrentSpend, spendingErr.LimitAmount)
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
		req = texting.InternalAIImageMessage
	}

	req = UserReq(persona, req)

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
				return texting.MsgInsufficientCredits, nil
			}
			logger.E("OpenRouter API error in vision", "code", e.Code, "message", e.Message, "http_status", e.HTTPStatusCode, tracing.InnerError, err)
			return "", err
		default:
			logger.E("Failed to visionify image", tracing.InnerError, err)
			return "", err
		}
	}

	tokens := response.Usage.TotalTokens
	cost := decimal.NewFromFloat(response.Usage.Cost)

	if err := v.usage.SaveUsage(logger, user.ID, chatID, cost, tokens); err != nil {
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
