package telegram

import (
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Poller struct {
	bot      *tgbotapi.BotAPI
	log      *tracing.Logger
	config   *PollerConfig
	diplomat *Diplomat
	handler  *TelegramHandler
}

func NewPoller(bot *tgbotapi.BotAPI, log *tracing.Logger, diplomat *Diplomat, config *PollerConfig) *Poller {
	return &Poller{bot: bot, log: log, diplomat: diplomat, config: config}
}

func (x *Poller) Start() {
	update := tgbotapi.NewUpdate(0)
	update.Timeout = x.config.Timeout
	update.AllowedUpdates = x.config.AllowedUpdates

	for update := range x.bot.GetUpdatesChan(update) {
		if msg := update.Message; msg != nil {
			user := update.SentFrom()

			log := x.log.With(
					tracing.UserId, user.ID,
					tracing.UserName, user.UserName,
					tracing.ChatType, msg.Chat.Type,
					tracing.ChatId, msg.Chat.ID,
					tracing.MessageId, msg.MessageID,
					tracing.MessageDate, msg.Date,
			)

			if err := x.handler.HandleMessage(log, msg); err != nil {
				x.diplomat.Reply(log, msg, texting.MsgXiError)
			}

			log.I("Message handled")
		}
	}
}

func (x *Poller) Stop() {
	x.bot.StopReceivingUpdates()
}