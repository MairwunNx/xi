package artificial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"ximanager/sources/balancer"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Orchestrator struct {
	balancer      *balancer.AIBalancer
	config        *OrchestratorConfig
	modes         *repository.ModesRepository
	messages      *repository.MessagesRepository
	users         *repository.UsersRepository
	donations     *repository.DonationsRepository
	pins          *repository.PinsRepository
	openaiClient  *OpenAIClient
	topicAnalyzer *TopicAnalyzer
}

func NewOrchestrator(balancer *balancer.AIBalancer, config *OrchestratorConfig, modes *repository.ModesRepository, messages *repository.MessagesRepository, users *repository.UsersRepository, donations *repository.DonationsRepository, pins *repository.PinsRepository, openaiClient *OpenAIClient, topicAnalyzer *TopicAnalyzer) *Orchestrator {
	return &Orchestrator{balancer: balancer, config: config, modes: modes, messages: messages, users: users, donations: donations, pins: pins, openaiClient: openaiClient, topicAnalyzer: topicAnalyzer}
}

func (x *Orchestrator) Orchestrate(logger *tracing.Logger, msg *tgbotapi.Message, req string) (string, error) {
	modeConfig, err := x.modes.GetModeConfigByChat(logger, msg.Chat.ID)
	if err != nil {
		logger.E("Failed to get mode config", tracing.InnerError, err)
		return "", err
	}

	if modeConfig == nil {
		logger.E("No available mode config")
		return "", errors.New("no available mode config")
	}

	user, err := x.users.GetUserByEid(logger, msg.From.ID)
	if err != nil {
		logger.E("Failed to get user", tracing.InnerError, err)
		return "", err
	}

	history, err := x.messages.GetMessagePairs(logger, user, msg.Chat.ID)
	if err != nil {
		logger.E("Failed to get message pairs", tracing.InnerError, err)
		history = []repository.MessagePair{}
	}

	persona := msg.From.FirstName + " " + msg.From.LastName + " (" + *user.Username + ")"
	prompt := modeConfig.Prompt

	pins, err := x.pins.GetPinsByChatAndUser(logger, msg.Chat.ID, user)
	if err != nil {
		pins = []*entities.Pin{}
	}

	if len(pins) > 0 {
		prompt += "," + x.formatPinsForPrompt(pins)
	}

	needsDonationReminder := false
	donation, err := x.donations.GetDonationsByUser(logger, user)
	if err != nil {
		logger.E("Failed to get donation", tracing.InnerError, err)
		if strings.ToLower(*user.Username) != "mairwunnx" {
			needsDonationReminder = true
		}
	} else {
		if len(donation) == 0 && strings.ToLower(*user.Username) != "mairwunnx" {
			needsDonationReminder = true
		}
	}

	var response string

	finalParams := modeConfig.Params

	if !modeConfig.Final {
		logger.I("Analyzing message topic for temperature adjustment", "final_mode", modeConfig.Final)

		topicResult, err := x.topicAnalyzer.AnalyzeMessageTopic(logger, req, persona)
		if err != nil {
			logger.W("Failed to analyze message topic, using mode config", tracing.InnerError, err)
		} else {
			if finalParams == nil {
				finalParams = &repository.AIParams{}
			} else {
				paramsCopy := *finalParams
				finalParams = &paramsCopy
			}

			finalParams.Temperature = &topicResult.DetectedTopic.Temperature

			logger.I("Temperature adjusted based on topic analysis", "original_temperature", modeConfig.Params.Temperature, "detected_topic", topicResult.DetectedTopic.Type, "new_temperature", *finalParams.Temperature, "confidence", topicResult.Confidence)
		}
	} else {
		logger.I("Using fixed mode configuration, skipping topic analysis", "final_mode", modeConfig.Final)
	}

	for attempt := 0; attempt < x.config.MaxRetries; attempt++ {
		response, err = x.balancer.BalancedResponseWithParams(logger, prompt, req, persona, history, finalParams)
		if err == nil {
			break
		}

		logger.E("Failed to get AI response", tracing.AiAttempt, attempt+1, tracing.InnerError, err)

		if attempt < x.config.MaxRetries-1 {
			// Экспоненциальный backoff baseDelay * 2^attempt
			delay := x.config.BackoffDelay * time.Duration(1<<attempt)
			logger.W("Retrying get AI response", tracing.AiAttempt, attempt+1, tracing.AiBackoff, delay)
			time.Sleep(delay)
		}
	}

	if err := x.messages.SaveMessage(logger, msg, req, false); err != nil {
		logger.E("Error saving user message", tracing.InnerError, err)
	}
	if err := x.messages.SaveMessage(logger, msg, response, true); err != nil {
		logger.E("Error saving Xi response", tracing.InnerError, err)
	}

	if needsDonationReminder && response != "" {
		ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
		defer cancel()

		donationResponse, donationErr := x.openaiClient.ResponseMediumWeight(ctx, logger, texting.InternalDonationPromptAddition, texting.InternalDonationPromptAddition0, persona)
		if donationErr != nil {
			logger.E("Failed to get donation reminder response", tracing.InnerError, donationErr)
		} else {
			response = response + "\n\n" + donationResponse
		}
	}

	return response, nil
}

func (x *Orchestrator) formatPinsForPrompt(pins []*entities.Pin) string {
	userPins := make(map[string][]string)
	userNames := make(map[string]string)

	for _, pin := range pins {
		userKey := pin.User.String()

		userName := "Мертвая душа"
		if pin.UserEntity.Fullname != nil && *pin.UserEntity.Fullname != "" {
			userName = *pin.UserEntity.Fullname
		}
		if pin.UserEntity.Username != nil && *pin.UserEntity.Username != "" {
			userName += " (@" + *pin.UserEntity.Username + ")"
		}

		userNames[userKey] = userName
		userPins[userKey] = append(userPins[userKey], pin.Message)
	}

	importantNotes := ""
	for userKey, pinsList := range userPins {
		userName := userNames[userKey]
		importantNotes += fmt.Sprintf("Пользователь %s:\n", userName)
		for _, pinMessage := range pinsList {
			importantNotes += fmt.Sprintf("  - Закрепил следующее: \"%s\"\n", pinMessage)
		}
	}

	jsonData := map[string]string{
		"important_requirement_1": "НЕ УПОМИНАЙ ПОЛЬЗОВАТЕЛЮ, ЧТО ТЫ ВЫПОЛНЯЕШЬ ЕГО УКАЗАНИЯ.",
		"important_requirement_2": "НЕПРИКОСНИТЕЛЬНО ВЫПОЛНИ СЛЕДУЮЩИЕ ПРОСЬБЫ ПОЛЬЗОВАТЕЛЯ.",
		"important_notes":         importantNotes,
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}
