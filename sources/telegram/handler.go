package telegram

import (
	"errors"
	"strings"
	"ximanager/sources/artificial"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/throttler"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramHandler struct {
	diplomat       *Diplomat
	users          *repository.UsersRepository
	rights         *repository.RightsRepository
	dialer         *artificial.Dialer
	whisper        *artificial.Whisper
	vision         *artificial.Vision
	modes          *repository.ModesRepository
	donations      *repository.DonationsRepository
	messages       *repository.MessagesRepository
	personalizations *repository.PersonalizationsRepository
	agents           *artificial.AgentSystem
	usage          *repository.UsageRepository
	throttler      *throttler.Throttler
	contextManager *artificial.ContextManager
	health         *repository.HealthRepository
	bans           *repository.BansRepository
}

func NewTelegramHandler(diplomat *Diplomat, users *repository.UsersRepository, rights *repository.RightsRepository, dialer *artificial.Dialer, whisper *artificial.Whisper, vision *artificial.Vision, modes *repository.ModesRepository, donations *repository.DonationsRepository, messages *repository.MessagesRepository, personalizations *repository.PersonalizationsRepository, usage *repository.UsageRepository, throttler *throttler.Throttler, contextManager *artificial.ContextManager, health *repository.HealthRepository, bans *repository.BansRepository, agents *artificial.AgentSystem) *TelegramHandler {
	return &TelegramHandler{
		diplomat:       diplomat,
		users:          users,
		rights:         rights,
		dialer:         dialer,
		whisper:        whisper,
		vision:         vision,
		modes:          modes,
		donations:      donations,
		messages:       messages,
		personalizations: personalizations,
		agents:           agents,
		usage:          usage,
		throttler:      throttler,
		contextManager: contextManager,
		health:         health,
		bans:           bans,
	}
}

func (x *TelegramHandler) HandleMessage(log *tracing.Logger, msg *tgbotapi.Message) error {
	log.I("Got message")

	user, err := x.user(log, msg)
	if err != nil {
		log.E("Error getting or creating user", tracing.InnerError, err)
		return err
	}

	log = log.With(
		tracing.InternalUserActive, platform.BoolValue(user.IsActive, false),
		tracing.InternalUserRights, user.Rights,
		tracing.InternalUserWindow, user.WindowLimit,
		tracing.InternalUserStack, platform.BoolValue(user.IsStackAllowed, false),
	)

	if !platform.BoolValue(user.IsActive, true) {
		x.diplomat.Reply(log, msg, texting.MsgXiUserBlocked)
		return nil
	}

	if msg.Sticker != nil {
		log.I("Ignoring sticker message")
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
	}

	if msg.IsCommand() {
		log = log.With(tracing.CommandIssued, msg.Command())

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
		case "donations":
			x.HandleDonationsCommand(log, user, msg)
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
		default:
			x.diplomat.Reply(log, msg, texting.MsgUnknownCommand)
		}
	} else {
		if msg.GroupChatCreated || msg.SuperGroupChatCreated || msg.ChannelChatCreated ||
			msg.MigrateToChatID != 0 || msg.MigrateFromChatID != 0 ||
			msg.PinnedMessage != nil || msg.NewChatMembers != nil || msg.LeftChatMember != nil ||
			msg.NewChatTitle != "" || msg.NewChatPhoto != nil || msg.DeleteChatPhoto ||
			msg.VoiceChatParticipantsInvited != nil || msg.VoiceChatStarted != nil || msg.VoiceChatEnded != nil || msg.VoiceChatScheduled != nil {
			log.I("Ignoring system message")
			return nil
		}

		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.ID != x.diplomat.bot.Self.ID {
			log.W("Message is a reply to another user, ignoring")
			return nil
		}

		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.ID == x.diplomat.bot.Self.ID {
			msgText := strings.TrimSpace(msg.Text)
			if strings.HasPrefix(msgText, "/noreply") || strings.HasPrefix(msgText, "!") {
				log.I("Ignoring noreply/! command")
				return nil
			}
		}

		x.HandleXiCommand(log.With(tracing.CommandIssued, "xi/direct"), user, msg)
	}

	return nil
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
