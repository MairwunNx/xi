package artificial

import (
	"context"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	"ximanager/sources/repository"

	"github.com/google/uuid"
	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
)

type Vision struct {
	ai     *openrouter.Client
	config *AIConfig
	usage  *repository.UsageRepository
}

func NewVision(config *AIConfig, ai *openrouter.Client, usage *repository.UsageRepository) *Vision {
	return &Vision{ai: ai, config: config, usage: usage}
}

func (v *Vision) Visionify(logger *tracing.Logger, iurl string, userID uuid.UUID, chatID int64, req string, persona string) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

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
		Model:    v.config.VisionPrimaryModel,
		Models:   v.config.VisionFallbackModels,
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

	if err := v.usage.SaveUsage(logger, userID, chatID, cost, tokens); err != nil {
		logger.E("Error saving usage", tracing.InnerError, err)
	}

	logger.I("vision completed", tracing.AiCost, cost.String(), tracing.AiTokens, tokens)

	return response.Choices[0].Message.Content.Text, nil
}
