package telegram

import (
	"errors"
	"strconv"
	"strings"
	"ximanager/sources/artificial"
	"ximanager/sources/features"
	"ximanager/sources/localization"
	"ximanager/sources/metrics"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/format"
	"ximanager/sources/texting/personality"
	"ximanager/sources/throttler"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramHandler struct {
	diplomat          *Diplomat
	users             *repository.UsersRepository
	rights            *repository.RightsRepository
	dialer            *artificial.Dialer
	whisper           *artificial.Whisper
	modes             *repository.ModesRepository
	donations         *repository.DonationsRepository
	messages          *repository.MessagesRepository
	personalizations  *repository.PersonalizationsRepository
	agents            *artificial.AgentSystem
	usage             *repository.UsageRepository
	throttler         *throttler.Throttler
	contextManager    *artificial.ContextManager
	health            *repository.HealthRepository
	bans              *repository.BansRepository
	broadcast         *repository.BroadcastRepository
	feedbacks         *repository.FeedbacksRepository
	tariffs           *repository.TariffsRepository
	chatState         *repository.ChatStateRepository
	features          *features.FeatureManager
	localization      *localization.LocalizationManager
	personality       *personality.XiPersonality
	dateTimeFormatter *format.DateTimeFormatter
	metrics           *metrics.MetricsService
}

func NewTelegramHandler(diplomat *Diplomat, users *repository.UsersRepository, rights *repository.RightsRepository, dialer *artificial.Dialer, whisper *artificial.Whisper, modes *repository.ModesRepository, donations *repository.DonationsRepository, messages *repository.MessagesRepository, personalizations *repository.PersonalizationsRepository, usage *repository.UsageRepository, throttler *throttler.Throttler, contextManager *artificial.ContextManager, health *repository.HealthRepository, bans *repository.BansRepository, broadcast *repository.BroadcastRepository, feedbacks *repository.FeedbacksRepository, tariffs *repository.TariffsRepository, chatState *repository.ChatStateRepository, agents *artificial.AgentSystem, fm *features.FeatureManager, localization *localization.LocalizationManager, personality *personality.XiPersonality, dateTimeFormatter *format.DateTimeFormatter, metrics *metrics.MetricsService) *TelegramHandler {
	return &TelegramHandler{
		diplomat:          diplomat,
		users:             users,
		rights:            rights,
		dialer:            dialer,
		whisper:           whisper,
		modes:             modes,
		donations:         donations,
		messages:          messages,
		personalizations:  personalizations,
		agents:            agents,
		usage:             usage,
		throttler:         throttler,
		contextManager:    contextManager,
		health:            health,
		bans:              bans,
		broadcast:         broadcast,
		feedbacks:         feedbacks,
		tariffs:           tariffs,
		chatState:         chatState,
		features:          fm,
		localization:      localization,
		personality:       personality,
		dateTimeFormatter: dateTimeFormatter,
		metrics:           metrics,
	}
}

func (x *TelegramHandler) HandleMessage(log *tracing.Logger, msg *tgbotapi.Message) error {
	defer tracing.ProfilePoint(log, "Telegram handler message completed", "telegram.handler.message")()
	log.I("Got message")

	user, err := x.user(log, msg)
	if err != nil {
		log.E("Error getting or creating user", tracing.InnerError, err)
		return err
	}

	log = log.With(
		tracing.InternalUserActive, platform.BoolValue(user.IsActive, false),
		tracing.InternalUserRights, user.Rights,
	)

	if !platform.BoolValue(user.IsActive, true) {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgXiUserBlocked"))
		return nil
	}

	if msg.Sticker != nil {
		log.I("Ignoring sticker message")
		x.metrics.RecordMessageIgnored("sticker")
		return nil
	}

	if msg.Photo != nil && len(msg.Photo) != 0 {
		x.HandleXiCommand(log.With(tracing.CommandIssued, "xi/photo"), user, msg)
		return nil
	}

	if msg.ReplyToMessage != nil && msg.IsCommand() && msg.Command() == "xi" {
		replyMsg := msg.ReplyToMessage
		if replyMsg.Voice != nil || replyMsg.VideoNote != nil || replyMsg.Audio != nil || replyMsg.Video != nil {
			x.XiCommandAudio(log.With(tracing.CommandIssued, "xi/audio"), user, msg, replyMsg)
			return nil
		}

		if replyMsg.Photo != nil && len(replyMsg.Photo) != 0 {
			x.XiCommandPhotoFromReply(log.With(tracing.CommandIssued, "xi/photo_reply"), user, msg, replyMsg)
			return nil
		}
	}

	if msg.IsCommand() {
		log = log.With(tracing.CommandIssued, msg.Command())
		x.metrics.RecordCommandUsed(msg.Command())

		switch msg.Command() {
		case "start":
			x.HandleStartCommand(log, user, msg)
		case "help":
			x.HandleHelpCommand(log, user, msg)
		case "xi":
			x.HandleXiCommand(log, user, msg)
		case "mode":
			x.HandleModeCommand(log, user, msg)
		case "users":
			x.HandleUsersCommand(log, user, msg)
		case "this":
			x.HandleThisCommand(log, user, msg)
		case "stats":
			x.HandleStatsCommand(log, user, msg)
		case "personalization":
			x.HandlePersonalizationCommand(log, user, msg)
		case "restart":
			x.HandleRestartCommand(log, user, msg)
		case "context":
			x.HandleContextCommand(log, user, msg)
		case "health":
			x.HandleHealthCommand(log, user, msg)
		case "ban":
			x.HandleBanCommand(log, user, msg)
		case "pardon":
			x.HandlePardonCommand(log, user, msg)
		case "tariff":
			x.HandleTariffCommand(log, user, msg)
		case "cancel":
			x.HandleCancelCommand(log, user, msg)
		default:
			x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUnknownCommand"))
		}
	} else {
		if handled := x.handleChatStateMessage(log, user, msg); handled {
			return nil
		}
		if msg.GroupChatCreated || msg.SuperGroupChatCreated || msg.ChannelChatCreated ||
			msg.MigrateToChatID != 0 || msg.MigrateFromChatID != 0 ||
			msg.PinnedMessage != nil || msg.NewChatMembers != nil || msg.LeftChatMember != nil ||
			msg.NewChatTitle != "" || msg.NewChatPhoto != nil || msg.DeleteChatPhoto ||
			msg.VoiceChatParticipantsInvited != nil || msg.VoiceChatStarted != nil || msg.VoiceChatEnded != nil || msg.VoiceChatScheduled != nil {
			log.I("Ignoring system message")
			x.metrics.RecordMessageIgnored("system")
			return nil
		}

		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.ID != x.diplomat.bot.Self.ID {
			log.W("Message is a reply to another user, ignoring")
			x.metrics.RecordMessageIgnored("reply_to_other")
			return nil
		}

		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.ID == x.diplomat.bot.Self.ID {
			msgText := strings.TrimSpace(msg.Text)
			if strings.HasPrefix(msgText, "/noreply") || strings.HasPrefix(msgText, "!") || strings.HasPrefix(msgText, ">") || strings.HasPrefix(msgText, "^") {
				log.I("Ignoring noreply/!/>/^ command")
				x.metrics.RecordMessageIgnored("noreply_prefix")
				return nil
			}
		}

		x.HandleXiCommand(log.With(tracing.CommandIssued, "xi/direct"), user, msg)
	}

	return nil
}

func (x *TelegramHandler) HandleCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery) error {
	defer tracing.ProfilePoint(log, "Telegram handler callback completed", "telegram.handler.callback")()
	log.I("Got callback", "data", query.Data)

	user, err := x.user(log, query.Message)
	if err != nil {
		log.E("Error getting or creating user", tracing.InnerError, err)
		return err
	}

	if query.Data == "unsubscribe_broadcast" {
		if *user.IsUnsubscribed {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgBroadcastAlreadyUnsubscribed"))
			if _, err := x.diplomat.bot.Request(callback); err != nil {
				log.E("Failed to answer callback", tracing.InnerError, err)
			}
			return nil
		}

		user.IsUnsubscribed = platform.BoolPtr(true)
		if _, err := x.users.UpdateUser(log, user); err != nil {
			log.E("Failed to update user", tracing.InnerError, err)
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgBroadcastErrorUnsubscribe"))
			if _, err := x.diplomat.bot.Request(callback); err != nil {
				log.E("Failed to answer callback", tracing.InnerError, err)
			}
			return err
		}

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgBroadcastUnsubscribed"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return nil
	}

	if strings.HasPrefix(query.Data, "feedback_like_") || strings.HasPrefix(query.Data, "feedback_dislike_") {
		x.handleFeedbackCallback(log, query, user)
		return nil
	}

	// Mode selection callbacks: mode_select_{modeType}
	if strings.HasPrefix(query.Data, "mode_select_") {
		x.handleModeSelectCallback(log, query, user)
		return nil
	}

	// Mode edit callbacks: mode_edit_{action}_{modeType}
	if strings.HasPrefix(query.Data, "mode_edit_") {
		x.handleModeEditCallback(log, query, user)
		return nil
	}

	// Mode delete confirmation callbacks: mode_delete_{confirm|cancel}_{modeType}
	if strings.HasPrefix(query.Data, "mode_delete_") {
		x.handleModeDeleteCallback(log, query, user)
		return nil
	}

	// Mode info callback: mode_info_{modeType}
	if strings.HasPrefix(query.Data, "mode_info_") {
		x.handleModeInfoCallback(log, query, user)
		return nil
	}

	// Context toggle callbacks: context_enable, context_disable
	if query.Data == "context_enable" || query.Data == "context_disable" {
		x.handleContextToggleCallback(log, query, user)
		return nil
	}

	// Context clear callback: context_clear
	if query.Data == "context_clear" {
		x.handleContextClearCallback(log, query, user)
		return nil
	}

	// Context clear confirmation callbacks: context_clear_confirm, context_clear_cancel
	if query.Data == "context_clear_confirm" || query.Data == "context_clear_cancel" {
		x.handleContextClearConfirmCallback(log, query, user)
		return nil
	}

	// Personalization callbacks: personalization_add, personalization_remove, personalization_print
	if query.Data == "personalization_add" || query.Data == "personalization_remove" || query.Data == "personalization_print" {
		x.handlePersonalizationCallback(log, query, user)
		return nil
	}

	// Personalization delete confirmation callbacks
	if query.Data == "personalization_delete_confirm" || query.Data == "personalization_delete_cancel" {
		x.handlePersonalizationDeleteCallback(log, query, user)
		return nil
	}

	// Broadcast confirmation callbacks
	if query.Data == "broadcast_send" || query.Data == "broadcast_cancel" {
		x.handleBroadcastConfirmCallback(log, query, user)
		return nil
	}

	// User action callbacks: user_enable_, user_disable_, user_delete_, user_rights_
	if strings.HasPrefix(query.Data, "user_enable_") || strings.HasPrefix(query.Data, "user_disable_") ||
		strings.HasPrefix(query.Data, "user_delete_") || strings.HasPrefix(query.Data, "user_rights_") {
		if !strings.Contains(query.Data, "_confirm_") && !strings.Contains(query.Data, "_cancel_") {
			x.handleUserActionCallback(log, query, user)
			return nil
		}
	}

	// User disable confirmation callbacks
	if strings.HasPrefix(query.Data, "user_disable_confirm_") || strings.HasPrefix(query.Data, "user_disable_cancel_") {
		x.handleUserDisableConfirmCallback(log, query, user)
		return nil
	}

	// User delete confirmation callbacks
	if strings.HasPrefix(query.Data, "user_delete_confirm_") || strings.HasPrefix(query.Data, "user_delete_cancel_") {
		x.handleUserDeleteConfirmCallback(log, query, user)
		return nil
	}

	// User rights toggle callbacks
	if strings.HasPrefix(query.Data, "user_right_") {
		x.handleUserRightsToggleCallback(log, query, user)
		return nil
	}

	// Pardon user callback: pardon_user_{userID}
	if strings.HasPrefix(query.Data, "pardon_user_") {
		x.handlePardonCallback(log, query, user)
		return nil
	}

	// Tariff add callback
	if query.Data == "tariff_add" {
		x.handleTariffAddCallback(log, query, user)
		return nil
	}

	// Tariff info callback: tariff_info_{key}
	if strings.HasPrefix(query.Data, "tariff_info_") {
		x.handleTariffInfoCallback(log, query, user)
		return nil
	}

	return nil
}

func (x *TelegramHandler) handleFeedbackCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	// Parse callback data format: feedback_{like|dislike}_{kind}_{userID}
	parts := strings.Split(query.Data, "_")
	if len(parts) != 4 {
		log.E("Invalid feedback callback data format", "data", query.Data)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgFeedbackError"))
		x.diplomat.bot.Request(callback)
		return
	}

	isLike := parts[1] == "like"
	kind := repository.FeedbackKind(parts[2])
	targetUserIDStr := parts[3]

	targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
	if err != nil {
		log.E("Failed to parse target user ID from callback", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgFeedbackError"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	if query.From.ID != targetUserID {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgFeedbackNotYourMessage"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	liked := 0
	if isLike {
		liked = 1
	}

	_, err = x.feedbacks.CreateFeedback(log, user.ID, liked, kind)
	if err != nil {
		log.E("Failed to create feedback", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgFeedbackError"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	feedbackType := "dislike"
	if isLike {
		feedbackType = "like"
	}
	x.metrics.RecordFeedback(feedbackType + "_" + string(kind))

	callbackText := x.localization.LocalizeBy(query.Message, "MsgFeedbackDisliked")
	if isLike {
		callbackText = x.localization.LocalizeBy(query.Message, "MsgFeedbackLiked")
	}

	callback := tgbotapi.NewCallback(query.ID, callbackText)
	if _, err := x.diplomat.bot.Request(callback); err != nil {
		log.E("Failed to answer callback", tracing.InnerError, err)
	}

	editMarkup := tgbotapi.NewEditMessageReplyMarkup(query.Message.Chat.ID, query.Message.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
	if _, err := x.diplomat.bot.Request(editMarkup); err != nil {
		log.W("Failed to remove feedback buttons", tracing.InnerError, err)
	}

	log.I("Feedback recorded", "user_id", user.ID, "liked", liked, "kind", kind)
}

func (x *TelegramHandler) user(log *tracing.Logger, msg *tgbotapi.Message) (*entities.User, error) {
	euid := msg.From.ID
	uname := msg.From.UserName
	fullname := msg.From.FirstName + " " + msg.From.LastName

	user, err := x.users.GetUserByEid(log, euid)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		log.E("Error getting user", tracing.InnerError, err)
		return nil, err
	}

	if user == nil {
		log.I("User not found, creating new user")
		user, err = x.users.CreateUser(log, euid, &uname, &fullname)
		if err != nil {
			log.E("Error creating user", tracing.InnerError, err)
			return nil, err
		}
	}

	return user, nil
}