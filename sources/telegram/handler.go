package telegram

import (
	"errors"
	"ximanager/sources/artificial"
	"ximanager/sources/balancer"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/throttler"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramHandler struct {
	diplomat     *Diplomat
	users        *repository.UsersRepository
	rights       *repository.RightsRepository
	orchestrator *artificial.Orchestrator
	modes        *repository.ModesRepository
	donations    *repository.DonationsRepository
	messages     *repository.MessagesRepository
	pins         *repository.PinsRepository
	throttler    *throttler.Throttler
	balancer     *balancer.AIBalancer
}

func NewTelegramHandler(diplomat *Diplomat, users *repository.UsersRepository, rights *repository.RightsRepository, orchestrator *artificial.Orchestrator, modes *repository.ModesRepository, donations *repository.DonationsRepository, messages *repository.MessagesRepository, pins *repository.PinsRepository, throttler *throttler.Throttler, balancer *balancer.AIBalancer) *TelegramHandler {
	return &TelegramHandler{
		diplomat:     diplomat,
		users:        users,
		rights:       rights,
		orchestrator: orchestrator,
		modes:        modes,
		donations:    donations,
		messages:     messages,
		pins:         pins,
		throttler:    throttler,
		balancer:     balancer,
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
		tracing.InternalUserActive, user.IsActive,
		tracing.InternalUserRights, user.Rights,
		tracing.InternalUserWindow, user.WindowLimit,
		tracing.InternalUserStack, user.IsStackAllowed,
	)

	if !user.IsActive {
		x.diplomat.Reply(log, msg, texting.MsgXiUserBlocked)
		return nil
	}

	if msg.Photo != nil && len(msg.Photo) != 0 {
		x.HandleXiCommand(log.With(tracing.CommandIssued, "xi/photo"), user, msg)
		return nil
	}

	if msg.ReplyToMessage != nil && msg.IsCommand() && msg.Command() == "xi" {
		replyMsg := msg.ReplyToMessage
		if replyMsg.Voice != nil || replyMsg.VideoNote != nil || replyMsg.Audio != nil || replyMsg.Video != nil {
			x.XiCommandAudio(log.With(tracing.CommandIssued, "xi/audio"), msg, replyMsg)
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
		case "context":
			x.HandleContextCommand(log, user, msg)
		case "pinned":
			x.HandlePinnedCommand(log, user, msg)
		case "wtf":
			x.HandleWtfCommand(log, user, msg)
		default:
			x.diplomat.Reply(log, msg, texting.MsgUnknownCommand)
		}
	} else {
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
