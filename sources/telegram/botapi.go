package telegram

import (
	"ximanager/sources/configuration"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	DefaultAPIEndpoint     = "https://api.telegram.org/bot%s/%s"
	DefaultFileAPIEndpoint = "https://api.telegram.org/file/bot%s/%s"
)

func NewBotAPI(log *tracing.Logger, config *configuration.Config) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(config.Telegram.BotToken)
	if err != nil {
		log.F("Failed to initialize telegram bot", tracing.InnerError, err)
	}

	if config.Telegram.APIEndpoint != "" {
		bot.SetAPIEndpoint(config.Telegram.APIEndpoint)
		log.I("Telegram bot initialized with custom API endpoint", "api_endpoint", config.Telegram.APIEndpoint)
	} else {
		log.I("Telegram bot initialized with default API endpoint")
	}

	return bot
}

func GetFileAPIEndpoint(config *configuration.Config) string {
	if config.Telegram.FileAPIEndpoint != "" {
		return config.Telegram.FileAPIEndpoint
	}
	return DefaultFileAPIEndpoint
}