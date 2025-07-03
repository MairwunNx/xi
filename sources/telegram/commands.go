package telegram

import (
	"fmt"
	"os"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (x *TelegramHandler) HandleXiCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.throttler.IsAllowed(msg.From.ID) {
		log.W("User exceeded rate throttler")
		x.diplomat.Reply(log, msg, texting.MsgThrottleExceeded)
		return
	}

	if msg.Photo != nil && len(msg.Photo) > 0 {
		x.XiCommandPhoto(log, msg)
		return
	}

	x.XiCommandText(log, msg)
}

func (x *TelegramHandler) HandleModeCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		if !x.rights.IsUserHasRight(log, user, "switch_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeNoAccess))
			return
		}
		x.ModeCommandSwitch(log, user, msg)
		return
	}

	var cmd ModeCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeHelpText))
		return
	}

	switch ctx.Command() {
	case "add <chatid> <type> <name> <config>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeModifyNoAccess))
			return
		}
		x.ModeCommandAdd(log, user, msg, int64(cmd.Add.ChatID), cmd.Add.Type, cmd.Add.Name, cmd.Add.Config)
	case "list <chatid>":
		if !x.rights.IsUserHasRight(log, user, "switch_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeNoAccess))
			return
		}
		x.ModeCommandList(log, msg, int64(cmd.List.ChatID))
	case "disable <chatid> <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeModifyNoAccess))
			return
		}
		x.ModeCommandDisable(log, msg, int64(cmd.Disable.ChatID), cmd.Disable.Type)
	case "enable <chatid> <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeModifyNoAccess))
			return
		}
		x.ModeCommandEnable(log, msg, int64(cmd.Enable.ChatID), cmd.Enable.Type)
	case "delete <chatid> <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeModifyNoAccess))
			return
		}
		x.ModeCommandDelete(log, msg, int64(cmd.Delete.ChatID), cmd.Delete.Type)
	case "edit <chatid> <type> <config>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeModifyNoAccess))
			return
		}
		x.ModeCommandEdit(log, msg, int64(cmd.Edit.ChatID), cmd.Edit.Type, cmd.Edit.Config)
	default:
		log.W("Unknown mode subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeHelpText))
	}
}

func (x *TelegramHandler) HandleUsersCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgUsersNoAccess))
		return
	}

	var cmd UsersCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgUsersHelpText))
		return
	}

	switch ctx.Command() {
	case "remove <username>":
		x.UsersCommandRemove(log, msg, cmd.Remove.Username)
	case "edit <username> <rights>":
		x.UsersCommandEdit(log, msg, cmd.Edit.Username, cmd.Edit.Rights)
	case "disable <username>":
		x.UsersCommandDisable(log, msg, cmd.Disable.Username)
	case "enable <username>":
		x.UsersCommandEnable(log, msg, cmd.Enable.Username)
	case "window <username> <limit>":
		x.UsersCommandWindow(log, msg, cmd.Window.Username, cmd.Window.Limit)
	case "stack <username> <action>":
		enabled := x.ParseBooleanArgument(cmd.Stack.Action)
		x.UsersCommandStack(log, msg, cmd.Stack.Username, enabled)
	default:
		log.W("Unknown users subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgUsersUnknownCommand))
	}
}

func (x *TelegramHandler) HandleDonationsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	var cmd DonationsCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgDonationsHelpText))
		return
	}

	switch ctx.Command() {
	case "add <username> <sum>":
		if !x.rights.IsUserHasRight(log, user, "manage_users") {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgDonationsNoAccess))
			return
		}
		x.DonationsCommandAdd(log, msg, cmd.Add.Username, cmd.Add.Sum)
	case "list":
		x.DonationsCommandList(log, msg)
	default:
		log.W("Unknown donations subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgDonationsUnknownCommand))
	}
}

func (x *TelegramHandler) HandleThisCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.Reply(log, msg, texting.XiifyManual(fmt.Sprintf(texting.MsgThisInfo, user.UserID, *user.Fullname, *user.Username, user.ID, user.Rights, msg.Chat.ID, msg.Chat.Type, msg.Chat.Title)))
}

func (x *TelegramHandler) HandleStatsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.StatsCommand(log, user, msg)
}

func (x *TelegramHandler) HandleContextCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	/*if msg.Chat.Type != "private" && !x.rights.IsUserHasRight(log, user, "manage_context") {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextNoAccess))
		return
	}*/

	var cmd ContextCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextHelpText))
		return
	}

	switch ctx.Command() {
	case "refresh", "refresh <chatid>":
		var chatID int64
		if cmd.Refresh.ChatID != nil {
			chatID = int64(*cmd.Refresh.ChatID)
		} else {
			chatID = msg.Chat.ID
		}
		x.ContextCommandRefresh(log, msg, chatID)
	case "disable":
		x.ContextCommandDisable(log, user, msg)
	case "enable":
		x.ContextCommandEnable(log, user, msg)
	default:
		log.W("Unknown context subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextHelpText))
	}
}

func (x *TelegramHandler) HandleRestartCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if user.Username != nil && *user.Username == "mairwunnx" {
		x.diplomat.Reply(log, msg, texting.MsgRestartText)
		os.Exit(0)
	}
}

func (x *TelegramHandler) HandleStartCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.Reply(log, msg, texting.MsgStartText)
}

func (x *TelegramHandler) HandlePinnedCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	var cmd PinnedCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedHelpText))
		return
	}

	switch ctx.Command() {
	case "add <message>":
		x.PinnedCommandAdd(log, user, msg, cmd.Add.Message)
	case "remove <message>":
		x.PinnedCommandRemove(log, user, msg, cmd.Remove.Message)
	case "list":
		x.PinnedCommandList(log, user, msg)
	default:
		log.W("Unknown pinned subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedHelpText))
	}
}

func (x *TelegramHandler) HandleHelpCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.Reply(log, msg, texting.MsgHelpText)
}
