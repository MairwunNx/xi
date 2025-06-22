package telegram

import (
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Diplomat struct {
	bot *tgbotapi.BotAPI
	config *DiplomatConfig
}

func NewDiplomat(bot *tgbotapi.BotAPI, config *DiplomatConfig) *Diplomat {
	return &Diplomat{bot: bot, config: config}
}

func (x *Diplomat) Reply(logger *tracing.Logger, msg *tgbotapi.Message, text string) {
	tracing.ReportExecution(logger,
		func() {
			for _, chunk := range texting.Chunks(text, x.config.ChunkSize) {
				
				if (msg.From.UserName == "mairwunnx" || msg.From.UserName == "lynfortune") { // TODO: remove
					logger.D("Sending message", "chunk", chunk)
					logger.D("Escaped message", "chunk", texting.EscapeMarkdown(chunk))
				}

				chattable := tgbotapi.NewMessage(msg.Chat.ID, texting.EscapeMarkdown(chunk))
				chattable.ReplyToMessageID = msg.MessageID
				chattable.ParseMode = tgbotapi.ModeMarkdownV2

				if _, err := x.bot.Send(chattable); err != nil {
					logger.E("Message chunk sending error", tracing.InnerError, err)
					emsg := tgbotapi.NewMessage(msg.Chat.ID, texting.EscapeMarkdown(texting.MsgXiError))
					emsg.ReplyToMessageID = msg.MessageID
					emsg.ParseMode = tgbotapi.ModeMarkdownV2

					if _, err := x.bot.Send(emsg); err != nil {
						logger.E("Failed to send fallback message", tracing.InnerError, err)
					}
					break
				}
			}
		}, func(l *tracing.Logger) { l.I("Message sent") },
	)
}