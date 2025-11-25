package telegram

import (
	"fmt"
	"strings"
	"ximanager/sources/configuration"
	"ximanager/sources/features"
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
	bot           *tgbotapi.BotAPI
	config        *configuration.Config
	users         *repository.UsersRepository
	donations     *repository.DonationsRepository
	localization  *localization.LocalizationManager
	metrics       *metrics.MetricsService
	features      *features.FeatureManager
	typingManager *TypingManager
}

func NewDiplomat(bot *tgbotapi.BotAPI, config *configuration.Config, users *repository.UsersRepository, donations *repository.DonationsRepository, localization *localization.LocalizationManager, metrics *metrics.MetricsService, fm *features.FeatureManager, log *tracing.Logger) *Diplomat {
	return &Diplomat{
		bot:           bot,
		config:        config,
		users:         users,
		donations:     donations,
		localization:  localization,
		metrics:       metrics,
		features:      fm,
		typingManager: NewTypingManager(bot, log),
	}
}

func (x *Diplomat) Reply(logger *tracing.Logger, msg *tgbotapi.Message, text string) {
	defer tracing.ProfilePoint(logger, "Diplomat reply completed", "diplomat.reply")()

	chunks := transform.Chunks(text, x.config.Telegram.DiplomatChunkSize)
	isXiResponse := strings.HasPrefix(text, x.localization.LocalizeBy(msg, "MsgXiResponse"))

	for i, chunk := range chunks {
		chattable := tgbotapi.NewMessage(msg.Chat.ID, markdown.EscapeMarkdownActor(chunk))
		chattable.ReplyToMessageID = msg.MessageID
		chattable.ParseMode = tgbotapi.ModeMarkdownV2

		isLastChunk := i == len(chunks)-1

		if isXiResponse && isLastChunk {
			var rows [][]tgbotapi.InlineKeyboardButton

			if x.features.IsEnabled(features.FeatureFeedbackButtons) {
				likeData := fmt.Sprintf("feedback_like_dialer_%d", msg.From.ID)
				dislikeData := fmt.Sprintf("feedback_dislike_dialer_%d", msg.From.ID)
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(x.localization.LocalizeBy(msg, "MsgFeedbackLikeEmoji"), likeData),
					tgbotapi.NewInlineKeyboardButtonData(x.localization.LocalizeBy(msg, "MsgFeedbackDislikeEmoji"), dislikeData),
				))
			}

			user, err := x.users.GetUserByEid(logger, msg.From.ID)
			if err != nil {
				logger.E("Failed to get user", tracing.InnerError, err)
			} else {
				grade, err := x.donations.GetUserGrade(logger, user)
				if err != nil {
					logger.E("Failed to get donations", tracing.InnerError, err)
				} else {
					if grade != platform.GradeGold && *user.Username != "mairwunnx" {
						rows = append(rows, tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonURL(x.localization.LocalizeBy(msg, "MsgDonationsSupport"), "https://www.tbank.ru/cf/3uoCqIOiT8V"),
						))
					}
				}
			}

			if len(rows) > 0 {
				chattable.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
			}
		}

		if _, err := x.bot.Send(chattable); err != nil {
			logger.E("Message chunk sending error", tracing.InnerError, err)
			x.metrics.RecordMessageSent("error")
			emsg := tgbotapi.NewMessage(msg.Chat.ID, markdown.EscapeMarkdownActor(x.localization.LocalizeBy(msg, "MsgXiError")))
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

// ReplyAudio sends a reply with whisper transcription and feedback buttons
func (x *Diplomat) ReplyAudio(logger *tracing.Logger, msg *tgbotapi.Message, text string) {
	defer tracing.ProfilePoint(logger, "Diplomat reply audio completed", "diplomat.reply_audio")()

	chunks := transform.Chunks(text, x.config.Telegram.DiplomatChunkSize)

	for i, chunk := range chunks {
		chattable := tgbotapi.NewMessage(msg.Chat.ID, markdown.EscapeMarkdownActor(chunk))
		chattable.ReplyToMessageID = msg.MessageID
		chattable.ParseMode = tgbotapi.ModeMarkdownV2

		isLastChunk := i == len(chunks)-1

		if isLastChunk && x.features.IsEnabled(features.FeatureFeedbackButtons) {
			likeData := fmt.Sprintf("feedback_like_whisper_%d", msg.From.ID)
			dislikeData := fmt.Sprintf("feedback_dislike_whisper_%d", msg.From.ID)
			rows := [][]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(x.localization.LocalizeBy(msg, "MsgFeedbackLikeEmoji"), likeData),
					tgbotapi.NewInlineKeyboardButtonData(x.localization.LocalizeBy(msg, "MsgFeedbackDislikeEmoji"), dislikeData),
				),
			}
			chattable.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		}

		if _, err := x.bot.Send(chattable); err != nil {
			logger.E("Audio message chunk sending error", tracing.InnerError, err)
			x.metrics.RecordMessageSent("error")
			emsg := tgbotapi.NewMessage(msg.Chat.ID, markdown.EscapeMarkdownActor(x.localization.LocalizeBy(msg, "MsgXiError")))
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

func (x *Diplomat) StartTyping(chatID int64) {
	x.typingManager.Start(chatID)
}

func (x *Diplomat) StopTyping(chatID int64) {
	x.typingManager.Stop(chatID)
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

type StreamingReply struct {
	diplomat      *Diplomat
	logger        *tracing.Logger
	originalMsg   *tgbotapi.Message
	chatID        int64
	replyToMsgID  int
	sentMessageID int
	userID        int64
}

func (x *Diplomat) StartStreamingReply(logger *tracing.Logger, msg *tgbotapi.Message) (*StreamingReply, error) {
	initialText := "▌"
	chattable := tgbotapi.NewMessage(msg.Chat.ID, initialText)
	chattable.ReplyToMessageID = msg.MessageID

	sent, err := x.bot.Send(chattable)
	if err != nil {
		logger.E("Failed to send initial streaming message", tracing.InnerError, err)
		return nil, err
	}

	return &StreamingReply{
		diplomat:      x,
		logger:        logger,
		originalMsg:   msg,
		chatID:        msg.Chat.ID,
		replyToMsgID:  msg.MessageID,
		sentMessageID: sent.MessageID,
		userID:        msg.From.ID,
	}, nil
}

func (sr *StreamingReply) Update(text string) error {
	if text == "" {
		text = "▌"
	}
	displayText := text + "▌"

	if len(displayText) > sr.diplomat.config.Telegram.DiplomatChunkSize {
		displayText = displayText[:sr.diplomat.config.Telegram.DiplomatChunkSize]
	}

	edit := tgbotapi.NewEditMessageText(sr.chatID, sr.sentMessageID, displayText)

	if _, err := sr.diplomat.bot.Send(edit); err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			sr.logger.W("Failed to update streaming message", tracing.InnerError, err)
			return err
		}
	}
	return nil
}

func (sr *StreamingReply) Finish(text string) error {
	finalText := markdown.EscapeMarkdownActor(text)

	if len(finalText) > sr.diplomat.config.Telegram.DiplomatChunkSize {
		finalText = finalText[:sr.diplomat.config.Telegram.DiplomatChunkSize]
	}

	edit := tgbotapi.NewEditMessageText(sr.chatID, sr.sentMessageID, finalText)
	edit.ParseMode = tgbotapi.ModeMarkdownV2

	var rows [][]tgbotapi.InlineKeyboardButton

	if sr.diplomat.features.IsEnabled(features.FeatureFeedbackButtons) {
		likeData := fmt.Sprintf("feedback_like_dialer_%d", sr.userID)
		dislikeData := fmt.Sprintf("feedback_dislike_dialer_%d", sr.userID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(sr.diplomat.localization.LocalizeBy(sr.originalMsg, "MsgFeedbackLikeEmoji"), likeData),
			tgbotapi.NewInlineKeyboardButtonData(sr.diplomat.localization.LocalizeBy(sr.originalMsg, "MsgFeedbackDislikeEmoji"), dislikeData),
		))
	}

	if len(rows) > 0 {
		edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	if _, err := sr.diplomat.bot.Send(edit); err != nil {
		sr.logger.E("Failed to finish streaming message", tracing.InnerError, err)
		sr.diplomat.metrics.RecordMessageSent("error")
		return err
	}

	sr.diplomat.metrics.RecordMessageSent("success")
	return nil
}

func (sr *StreamingReply) FinishWithError(errorText string) error {
	edit := tgbotapi.NewEditMessageText(sr.chatID, sr.sentMessageID, errorText)

	if _, err := sr.diplomat.bot.Send(edit); err != nil {
		sr.logger.E("Failed to finish streaming message with error", tracing.InnerError, err)
		return err
	}

	sr.diplomat.metrics.RecordMessageSent("error")
	return nil
}

func (sr *StreamingReply) GetMessageID() int {
	return sr.sentMessageID
}