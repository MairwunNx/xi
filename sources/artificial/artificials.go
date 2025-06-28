package artificial

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"

	"ximanager/sources/balancer"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	deepseek "github.com/cohesion-org/deepseek-go"
	constants "github.com/cohesion-org/deepseek-go/constants"
	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/sashabaranov/go-openai"
)

type (
	OpenAIClient struct {
		client *openai.Client
		config *AIConfig
	}
	DeepseekClient struct {
		client *deepseek.Client
		config *AIConfig
	}
	GrokClient struct {
		client *openai.Client
		config *AIConfig
	}
	AnthropicClient struct {
		client *anthropic.Client
		config *AIConfig
	}
)

func NewOpenAIClient(client *http.Client, config *AIConfig) *OpenAIClient {
	openaiConfig := openai.DefaultConfig(config.OpenAIToken)
	openaiConfig.HTTPClient = client
	return &OpenAIClient{client: openai.NewClientWithConfig(openaiConfig), config: config}
}

func NewDeepseekClient(config *AIConfig) *DeepseekClient {
	return &DeepseekClient{client: deepseek.NewClient(config.DeepseekToken), config: config}
}

func NewGrokClient(client *http.Client, config *AIConfig) *GrokClient {
	grokOpenAIClientConfig := openai.DefaultConfig(config.GrokToken)
	grokOpenAIClientConfig.HTTPClient = client
	grokOpenAIClientConfig.BaseURL = "https://api.x.ai/v1"
	return &GrokClient{client: openai.NewClientWithConfig(grokOpenAIClientConfig), config: config}
}

func NewAnthropicClient(client *http.Client, config *AIConfig) *AnthropicClient {
	return &AnthropicClient{
		client: anthropic.NewClient(
			config.AnthropicToken,
			anthropic.WithHTTPClient(client),
		), config: config}
}

func NewAIClientsMap(openai *OpenAIClient, deepseek *DeepseekClient, grok *GrokClient, anthropic *AnthropicClient) map[string]balancer.NeuroProvider {
	return map[string]balancer.NeuroProvider{"openai": openai, "claude": anthropic, "deepseek": deepseek, "grok": grok}
}

func (p *OpenAIClient) Response(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair) (string, error) {
	return p.ResponseWithParams(ctx, log, prompt, req, persona, history, nil)
}

func (p *OpenAIClient) ResponseWithParams(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair, params *repository.AIParams) (string, error) {
	req = UserReq(persona, req)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: prompt},
	}

	if len(history) > 0 {
		historyMessages := MessagePairsToOpenAI(history)
		messages = append(messages, historyMessages...)
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: req,
	})

	at := texting.TokensInfer(log, prompt, req, persona, p.config.OpenAIMaxTokens)

	model := p.config.OpenAIModel
	request := openai.ChatCompletionRequest{
		Model:               model,
		Messages:            messages,
		MaxCompletionTokens: at,
	}

	log.I("prompt ResponseWithParams", "prompt", messages[0].Content)

	if params != nil {
		if params.TopP != nil {
			request.TopP = *params.TopP
		}
		if params.PresencePenalty != nil {
			request.PresencePenalty = *params.PresencePenalty
		}
		if params.FrequencyPenalty != nil {
			request.FrequencyPenalty = *params.FrequencyPenalty
		}
		if params.Temperature != nil {
			request.Temperature = *params.Temperature
		}
	}

	log.I("ai action requested", tracing.AiKind, "openai", tracing.AiModel, model, tracing.AiTokens, at, "history_pairs", len(history), "total_messages", len(messages))

	if response, err := p.client.CreateChatCompletion(ctx, request); err != nil {
		return "", err
	} else {
		return response.Choices[0].Message.Content, nil
	}
}

func (p *OpenAIClient) ResponseImage(ctx context.Context, log *tracing.Logger, iurl string, req string, persona string) (string, error) {
	return p.ResponseImageWithParams(ctx, log, iurl, req, persona, nil)
}

func (p *OpenAIClient) ResponseImageWithParams(ctx context.Context, log *tracing.Logger, iurl string, req string, persona string, params *repository.AIParams) (string, error) {
	if req == "" {
		req = texting.InternalAIImageMessage
	}

	req = UserReq(persona, req)

	const bias = (3 << 4) ^ 2
	mt := p.config.OpenAIImageMaxTokens
	const estimate = 1300

	pt := texting.Tokens(log, req)
	at := mt - pt - estimate - bias

	if at < 1300 {
		at = 1300
	}

	model := p.config.OpenAIImageModel
	request := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: openai.ChatMessagePartTypeText, Text: req},
					{
						Type:     openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{URL: iurl, Detail: openai.ImageURLDetailAuto},
					},
				},
			},
		},
		MaxCompletionTokens: at,
	}

	if params != nil {
		if params.TopP != nil {
			request.TopP = *params.TopP
		}
		if params.PresencePenalty != nil {
			request.PresencePenalty = *params.PresencePenalty
		}
		if params.FrequencyPenalty != nil {
			request.FrequencyPenalty = *params.FrequencyPenalty
		}
		if params.Temperature != nil {
			request.Temperature = *params.Temperature
		}
	}

	log.I("ai response requested", tracing.AiKind, "openai/image", tracing.AiModel, model, tracing.AiTokens, at)

	if response, err := p.client.CreateChatCompletion(ctx, request); err != nil {
		return "", err
	} else {
		return response.Choices[0].Message.Content, nil
	}
}

func (x *OpenAIClient) ResponseAudio(ctx context.Context, log *tracing.Logger, audioFile *os.File, prompt string) (string, error) {
	request := openai.AudioRequest{
		Model:    x.config.OpenAIAudioModel,
		FilePath: audioFile.Name(),
		Prompt:   prompt,
		Language: "ru",
	}

	log.I("speech-to-text requested", tracing.AiKind, "openai/whisper", tracing.AiModel, openai.Whisper1)

	response, err := x.client.CreateTranscription(ctx, request)
	if err != nil {
		log.E("Failed to transcribe audio", tracing.InnerError, err)
		return "", err
	}

	log.I("speech-to-text completed", "transcription_length", len(response.Text))
	return response.Text, nil
}

func (x *OpenAIClient) ResponseLightweight(ctx context.Context, log *tracing.Logger, transcriptedText string, userPrompt string, persona string) (string, error) {
	req := fmt.Sprintf("%s\n\n%s:\n%s", userPrompt, texting.InternalAIAudioPrompt, transcriptedText)
	req = UserReq(persona, req)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: req},
	}

	model := x.config.OpenAILightweightModel
	mt := texting.TokensInfer(log, userPrompt, req, persona, x.config.OpenAILightweightMaxTokens)
	request := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		MaxTokens: mt,
	}

	log.I("ai action requested", tracing.AiKind, "openai/gpt35", tracing.AiModel, model, tracing.AiTokens, 1000)

	response, err := x.client.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	return response.Choices[0].Message.Content, nil
}

func (x *OpenAIClient) ResponseMediumWeight(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string) (string, error) {
	model := x.config.OpenAIMediumWeightModel
	mt := texting.TokensInfer(log, prompt, req, "", x.config.OpenAIMediumWeightMaxTokens)
	request := openai.ChatCompletionRequest{
		Model:    model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: prompt},
			{Role: openai.ChatMessageRoleUser, Content: req},
		},
		MaxTokens: mt,
	}

	log.I("ai action requested", tracing.AiKind, "openai/gpt4o-mini", tracing.AiModel, model, tracing.AiTokens, mt)

	response, err := x.client.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	return response.Choices[0].Message.Content, nil
}

func (x *DeepseekClient) Response(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair) (string, error) {
	return x.ResponseWithParams(ctx, log, prompt, req, persona, history, nil)
}

func (x *DeepseekClient) ResponseWithParams(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair, params *repository.AIParams) (string, error) {
	req = UserReq(persona, req)

	messages := []deepseek.ChatCompletionMessage{
		{Role: constants.ChatMessageRoleSystem, Content: prompt},
	}

	if len(history) > 0 {
		historyMessages := MessagePairsToDeepseek(history)
		messages = append(messages, historyMessages...)
	}

	messages = append(messages, deepseek.ChatCompletionMessage{
		Role:    constants.ChatMessageRoleUser,
		Content: req,
	})

	at := texting.TokensInfer(log, prompt, req, persona, x.config.DeepseekMaxTokens)

	model := x.config.DeepseekModel
	request := &deepseek.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: at,
	}

	if params != nil {
		if params.TopP != nil {
			request.TopP = *params.TopP
		}
		if params.PresencePenalty != nil {
			request.PresencePenalty = *params.PresencePenalty
		}
		if params.FrequencyPenalty != nil {
			request.FrequencyPenalty = *params.FrequencyPenalty
		}
		if params.Temperature != nil {
			request.Temperature = *params.Temperature
		}
	}

	log.I("ai action requested", tracing.AiKind, "deepseek", tracing.AiModel, model, tracing.AiTokens, at, "history_pairs", len(history), "total_messages", len(messages))

	if response, err := x.client.CreateChatCompletion(ctx, request); err != nil {
		return "", err
	} else {
		return response.Choices[0].Message.Content, nil
	}
}

func (p *AnthropicClient) Response(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair) (string, error) {
	return p.ResponseWithParams(ctx, log, prompt, req, persona, history, nil)
}

func (p *AnthropicClient) ResponseWithParams(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair, params *repository.AIParams) (string, error) {
	req = UserReq(persona, req)
	model := p.config.AnthropicModel
	at := texting.TokensInfer(log, prompt, req, persona, p.config.AnthropicMaxTokens)

	var messages []anthropic.Message
	if len(history) > 0 {
		messages = MessagePairsToAnthropic(history)
	}

	messages = append(messages, anthropic.NewUserTextMessage(req))

	log.I("ai action requested", tracing.AiKind, "anthropic", tracing.AiModel, model, tracing.AiTokens, at, "history_pairs", len(history), "total_messages", len(messages))

	var thinking *anthropic.Thinking = nil

	if at > 2048 {
		thinking = &anthropic.Thinking{Type: anthropic.ThinkingTypeEnabled, BudgetTokens: at / 2}
	}

	requestParams := anthropic.MessagesRequest{
		Model:     anthropic.Model(model),
		System:    prompt,
		Messages:  messages,
		MaxTokens: at,
		Thinking: thinking,
	}

	if params != nil {
		if params.TopP != nil {
			requestParams.TopP = params.TopP
		}
		if params.TopK != nil {
			requestParams.TopK = params.TopK
		}
		if params.Temperature != nil {
			temp := float32(math.Min(1.0, float64(*params.Temperature)))
			requestParams.Temperature = &temp
		}
		if params.PresencePenalty != nil || params.FrequencyPenalty != nil {
			log.W("Anthropic ignores presence_penalty and frequency_penalty parameters")
		}
	}

	if resp, err := p.client.CreateMessages(ctx, requestParams); err != nil {
		return "", err
	} else {
		return resp.Content[0].GetText(), nil
	}
}

func (p *GrokClient) Response(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair) (string, error) {
	return p.ResponseWithParams(ctx, log, prompt, req, persona, history, nil)
}

func (p *GrokClient) ResponseWithParams(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair, params *repository.AIParams) (string, error) {
	req = UserReq(persona, req)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: prompt},
	}

	if len(history) > 0 {
		historyMessages := MessagePairsToOpenAI(history)
		messages = append(messages, historyMessages...)
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: req,
	})

	at := texting.TokensInfer(log, prompt, req, persona, p.config.GrokMaxTokens)

	model := p.config.GrokModel
	request := openai.ChatCompletionRequest{
		Model:               model,
		Messages:            messages,
		MaxCompletionTokens: at,
	}

	if params != nil {
		if params.TopP != nil {
			request.TopP = *params.TopP
		}
		if params.PresencePenalty != nil {
			request.PresencePenalty = *params.PresencePenalty
		}
		if params.FrequencyPenalty != nil {
			request.FrequencyPenalty = *params.FrequencyPenalty
		}
		if params.Temperature != nil {
			request.Temperature = *params.Temperature
		}
	}

	log.I("ai action requested", tracing.AiKind, "grok", tracing.AiModel, model, tracing.AiTokens, at, "history_pairs", len(history), "total_messages", len(messages))

	if response, err := p.client.CreateChatCompletion(ctx, request); err != nil {
		return "", err
	} else {
		return response.Choices[0].Message.Content, nil
	}
}

func UserReq(persona string, req string) string {
	return fmt.Sprintf(texting.InternalAIGreetingMessage, persona, req)
}
