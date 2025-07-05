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
	ai     *openrouter.Client
	config *AIConfig
	usage  *repository.UsageRepository
	limiter *SpendingLimiter
	users  *repository.UsersRepository
}

func NewVision(config *AIConfig, ai *openrouter.Client, usage *repository.UsageRepository, limiter *SpendingLimiter, users *repository.UsersRepository) *Vision {
	return &Vision{ai: ai, config: config, usage: usage, limiter: limiter, users: users}
}

func (v *Vision) Visionify(logger *tracing.Logger, iurl string, user *entities.User, chatID int64, req string, persona string) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	limitErr := v.limiter.CheckSpendingLimits(logger, user)
	modelToUse := v.config.VisionPrimaryModel
	fallbackModels := v.config.VisionFallbackModels
	var limitWarning string
	
	if limitErr != nil {
		if spendingErr, ok := limitErr.(*SpendingLimitExceededError); ok {
			logger.W("Spending limit exceeded for vision, using fallback model", "user_grade", spendingErr.UserGrade, "limit_type", spendingErr.LimitType, "limit", spendingErr.LimitAmount, "spent", spendingErr.CurrentSpend)
			
			modelToUse = v.config.LimitExceededModel
			fallbackModels = v.config.LimitExceededFallbackModels
			
			limitTypeText := texting.MsgSpendingLimitExceededDaily
			if spendingErr.LimitType == LimitTypeMonthly {
				limitTypeText = texting.MsgSpendingLimitExceededMonthly
			}
			limitWarning = fmt.Sprintf(texting.MsgSpendingLimitExceededVision, limitTypeText, spendingErr.UserGrade, spendingErr.CurrentSpend, spendingErr.LimitAmount)
		}
	}

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

	logger.I("vision completed", tracing.AiCost, cost.String(), tracing.AiTokens, tokens)

	responseText := response.Choices[0].Message.Content.Text
	
	if limitWarning != "" {
		responseText += limitWarning
	}

	return responseText, nil
}
