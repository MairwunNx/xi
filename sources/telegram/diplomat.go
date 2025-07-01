package telegram

import (
	"strings"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Diplomat struct {
	bot *tgbotapi.BotAPI
	config *DiplomatConfig
	users *repository.UsersRepository
	donations *repository.DonationsRepository
}

func NewDiplomat(bot *tgbotapi.BotAPI, config *DiplomatConfig, users *repository.UsersRepository, donations *repository.DonationsRepository) *Diplomat {
	return &Diplomat{bot: bot, config: config, users: users, donations: donations}
}

func (x *Diplomat) Reply(logger *tracing.Logger, msg *tgbotapi.Message, text string) {
	tracing.ReportExecution(logger,
		func() {
			for _, chunk := range texting.Chunks(text, x.config.ChunkSize) {
				chattable := tgbotapi.NewMessage(msg.Chat.ID, texting.EscapeMarkdown(chunk))
				chattable.ReplyToMessageID = msg.MessageID
				chattable.ParseMode = tgbotapi.ModeMarkdownV2

				if strings.HasPrefix(text, texting.MsgXiResponse) {
					user, err := x.users.GetUserByEid(logger, msg.From.ID)
					if err != nil {
						logger.E("Failed to get user", tracing.InnerError, err)
					} else {
						grade, err := x.donations.GetUserGrade(logger, user)
						if err != nil {
							logger.E("Failed to get donations", tracing.InnerError, err)
						} else {
							if grade != repository.UserGradeGold && *user.Username != "mairwunnx" {
								chattable.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
									tgbotapi.NewInlineKeyboardRow(
										tgbotapi.NewInlineKeyboardButtonURL("Поддержать проект ❤️‍🔥", "https://www.tbank.ru/cf/3uoCqIOiT8V"),
									),
								)
							}
						}
					}
				}

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

func (x *Diplomat) SendTyping(logger *tracing.Logger, chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	if _, err := x.bot.Send(action); err != nil {
		logger.W("Failed to send typing action", tracing.InnerError, err)
	}
}