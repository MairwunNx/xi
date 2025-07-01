package artificial

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openrouter "github.com/revrost/go-openrouter"
	"github.com/shopspring/decimal"
)

type Dialer struct {
	ai       *openrouter.Client
	config   *AIConfig
	modes    *repository.ModesRepository
	users    *repository.UsersRepository
	messages *repository.MessagesRepository
	pins     *repository.PinsRepository
}

func NewDialer(config *AIConfig, ai *openrouter.Client, modes *repository.ModesRepository, users *repository.UsersRepository, messages *repository.MessagesRepository, pins *repository.PinsRepository) *Dialer {
	return &Dialer{ai: ai, config: config, modes: modes, users: users, messages: messages, pins: pins}
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

	req = UserReq(persona, req)
	prompt := mode.Prompt

	history := []repository.MessagePair{}
	if stackful {
		history, err = x.messages.GetMessagePairs(log, user, msg.Chat.ID)
		if err != nil {
			log.E("Failed to get message pairs", tracing.InnerError, err)
			history = []repository.MessagePair{}
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

	if len(history) > 0 {
		historyMessages := OpenRouterMessageStackFrom(history)
		messages = append(messages, historyMessages...)
	}

	messages = append(messages, openrouter.ChatCompletionMessage{
		Role:    openrouter.ChatMessageRoleUser,
		Content: openrouter.Content{Text: req},
	})

	request := openrouter.ChatCompletionRequest{
		Model:     x.config.DialerPrimaryModel,
		Models:    x.config.DialerFallbackModels,
		Messages:  messages,
		Reasoning: &openrouter.ChatCompletionReasoning{Effort: openrouter.String(x.config.DialerReasoningEffort)},
		Usage:     &openrouter.IncludeUsage{Include: true},
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

	log = log.With("ai requested", tracing.AiKind, "openrouter/variable", tracing.AiModel, request.Model)

	response, err := x.ai.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	tokens := response.Usage.TotalTokens
	cost := decimal.NewFromFloat(response.Usage.Cost)

	log.I("ai completed", tracing.AiCost, cost.String(), tracing.AiTokens, tokens)

	responseText := response.Choices[0].Message.Content.Text

	if err := x.messages.SaveMessage(log, msg, req, false, cost, tokens); err != nil {
		log.E("Error saving user message", tracing.InnerError, err)
	}
	if err := x.messages.SaveMessage(log, msg, responseText, true, cost, tokens); err != nil {
		log.E("Error saving Xi response", tracing.InnerError, err)
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
