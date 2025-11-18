package telegram

import (
	"ximanager/sources/configuration"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func NewBotAPI(log *tracing.Logger, config *configuration.Config) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(config.Telegram.BotToken)
	if err != nil {
		log.F("Failed to initialize telegram bot", tracing.InnerError, err)
	} else {
		log.I("Telegram bot initialized")
	}
	return bot
}