package texting

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func EscapeMarkdown(input string) string {
	return tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, input)
}