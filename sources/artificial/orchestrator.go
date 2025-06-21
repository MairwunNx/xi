package artificial

import (
	"context"
	"errors"
	"strings"
	"time"
	"ximanager/sources/balancer"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Orchestrator struct {
	balancer *balancer.AIBalancer
	config   *OrchestratorConfig
	modes    *repository.ModesRepository
	messages *repository.MessagesRepository
	users    *repository.UsersRepository
	donations *repository.DonationsRepository
	openaiClient *OpenAIClient
}

func NewOrchestrator(balancer *balancer.AIBalancer, config *OrchestratorConfig, modes *repository.ModesRepository, messages *repository.MessagesRepository, users *repository.UsersRepository, donations *repository.DonationsRepository, openaiClient *OpenAIClient) *Orchestrator {
	return &Orchestrator{balancer: balancer, config: config, modes: modes, messages: messages, users: users, donations: donations, openaiClient: openaiClient}
}

func (x *Orchestrator) Orchestrate(logger *tracing.Logger, msg *tgbotapi.Message, req string) (string, error) {
	mode, err := x.modes.GetModeByChat(logger, msg.Chat.ID)
	if err != nil {
		logger.E("Failed to get current mode", tracing.InnerError, err)
		return "", err
	}

	if mode == nil {
		logger.E("No available modes")
		return "", errors.New("no available modes")
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
	prompt := mode.Prompt + texting.InternalQualifierPromptAddition

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

	for attempt := 0; attempt < x.config.MaxRetries; attempt++ {
		response, err = x.balancer.BalancedResponse(logger, prompt, req, persona, history)
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
		
		donationResponse, donationErr := x.openaiClient.ResponseMediumWeight(ctx, logger, texting.InternalDonationPromptAddition, response, persona)
		if donationErr != nil {
			logger.E("Failed to get donation reminder response", tracing.InnerError, donationErr)
		} else {
			response = donationResponse
		}
	}

	return response, nil
}
