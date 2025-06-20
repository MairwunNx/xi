package telegram

import (
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func NewBotAPI(log *tracing.Logger, config *BotConfig) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(config.Token)
	if err != nil {
		log.F("Failed to initialize telegram bot", tracing.InnerError, err)
	} else {
		log.I("Telegram bot initialized")
	}
	return bot
}