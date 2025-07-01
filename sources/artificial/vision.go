package artificial

import (
	"context"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
)

type Vision struct {
	ai *openrouter.Client
	config *AIConfig
}

func NewVision(config *AIConfig, ai *openrouter.Client) *Vision {
	return &Vision{ai: ai, config: config}
}

func (v *Vision) Visionify(logger *tracing.Logger, iurl string, req string, persona string) (string, error) {
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
		Model: v.config.VisionPrimaryModel,
		Models: v.config.VisionFallbackModels,
		Messages: messages,
		Usage:    &openrouter.IncludeUsage{Include: true},
	}

	logger = logger.With(tracing.AiKind, "openrouter/vision", tracing.AiModel, request.Model)

	logger.I("vision requested")

	response, err := v.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		logger.E("Failed to visionify image", tracing.InnerError, err)
		return "", err
	}

	tokens := response.Usage.TotalTokens
	cost := decimal.NewFromFloat(response.Usage.Cost)

	logger.I("vision completed", tracing.AiCost, cost.String(), tracing.AiTokens, tokens)

	return response.Choices[0].Message.Content.Text, nil
}
