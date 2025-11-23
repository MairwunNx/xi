package telegram

import (
	"strings"
	"ximanager/sources/configuration"
	"ximanager/sources/localization"
	"ximanager/sources/metrics"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/markdown"
	"ximanager/sources/texting/transform"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Diplomat struct {
	bot       *tgbotapi.BotAPI
	config    *configuration.Config
	users     *repository.UsersRepository
	donations *repository.DonationsRepository
	localization *localization.LocalizationManager
	metrics      *metrics.MetricsService
}

func NewDiplomat(bot *tgbotapi.BotAPI, config *configuration.Config, users *repository.UsersRepository, donations *repository.DonationsRepository, localization *localization.LocalizationManager, metrics *metrics.MetricsService) *Diplomat {
	return &Diplomat{bot: bot, config: config, users: users, donations: donations, localization: localization, metrics: metrics}
}

func (x *Diplomat) Reply(logger *tracing.Logger, msg *tgbotapi.Message, text string) {
  defer tracing.ProfilePoint(logger, "Diplomat reply completed", "diplomat.reply")()

	for _, chunk := range transform.Chunks(text, x.config.Telegram.DiplomatChunkSize) {
		chattable := tgbotapi.NewMessage(msg.Chat.ID, markdown.EscapeMarkdownActor(chunk))
		chattable.ReplyToMessageID = msg.MessageID
		chattable.ParseMode = tgbotapi.ModeMarkdownV2

		if strings.HasPrefix(text, x.localization.LocalizeBy(msg, "MsgXiResponse")) {
			user, err := x.users.GetUserByEid(logger, msg.From.ID)
			if err != nil {
				logger.E("Failed to get user", tracing.InnerError, err)
			} else {
				grade, err := x.donations.GetUserGrade(logger, user)
				if err != nil {
					logger.E("Failed to get donations", tracing.InnerError, err)
				} else {
					if grade != platform.GradeGold && *user.Username != "mairwunnx" {
						chattable.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
							tgbotapi.NewInlineKeyboardRow(
								tgbotapi.NewInlineKeyboardButtonURL(x.localization.LocalizeBy(msg, "MsgDonationsSupport"), "https://www.tbank.ru/cf/3uoCqIOiT8V"),
							),
						)
					}
				}
			}
		}

		if _, err := x.bot.Send(chattable); err != nil {
			logger.E("Message chunk sending error", tracing.InnerError, err)
			x.metrics.RecordMessageSent("error")
			emsg := tgbotapi.NewMessage(msg.Chat.ID, markdown.EscapeMarkdownActor(x.localization.LocalizeBy(msg, "MsgXiError")));
			emsg.ReplyToMessageID = msg.MessageID
			emsg.ParseMode = tgbotapi.ModeMarkdownV2

			if _, err := x.bot.Send(emsg); err != nil {
				logger.E("Failed to send fallback message", tracing.InnerError, err)
			}
			break
		}
		x.metrics.RecordMessageSent("success")
	}
}

func (x *Diplomat) SendTyping(logger *tracing.Logger, chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	if _, err := x.bot.Send(action); err != nil {
		logger.W("Failed to send typing action", tracing.InnerError, err)
	}
}

func (x *Diplomat) SendText(logger *tracing.Logger, chatID int64, text string) error {
	defer tracing.ProfilePoint(logger, "Diplomat send text completed", "diplomat.send_text")()

	for _, chunk := range transform.Chunks(text, x.config.Telegram.DiplomatChunkSize) {
		msg := tgbotapi.NewMessage(chatID, markdown.EscapeMarkdownActor(chunk))
		msg.ParseMode = tgbotapi.ModeMarkdownV2

		if _, err := x.bot.Send(msg); err != nil {
			logger.E("Message chunk sending error", tracing.InnerError, err)
			x.metrics.RecordMessageSent("error")
			return err
		}
		x.metrics.RecordMessageSent("success")
	}
	return nil
}

func (x *Diplomat) SendBroadcastMessage(logger *tracing.Logger, chatID int64, text string, unsubscribeText string) error {
	defer tracing.ProfilePoint(logger, "Diplomat send broadcast message completed", "diplomat.send_broadcast_message")()

	chunks := transform.Chunks(text, x.config.Telegram.DiplomatChunkSize)
	for i, chunk := range chunks {
		msg := tgbotapi.NewMessage(chatID, markdown.EscapeMarkdownActor(chunk))
		msg.ParseMode = tgbotapi.ModeMarkdownV2

		if i == len(chunks)-1 {
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(unsubscribeText, "unsubscribe_broadcast"),
				),
			)
		}

		if _, err := x.bot.Send(msg); err != nil {
			logger.E("Broadcast message chunk sending error", tracing.InnerError, err)
			x.metrics.RecordMessageSent("error")
			return err
		}
		x.metrics.RecordMessageSent("success")
	}
	return nil
}