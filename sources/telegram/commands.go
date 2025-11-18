package telegram

import (
	"os"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (x *TelegramHandler) HandleXiCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.throttler.IsAllowed(msg.From.ID) {
		log.W("User exceeded rate throttler")
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgThrottleExceeded"))
		return
	}

	ban, expiresAt, err := x.bans.GetActiveBanWithExpiry(log, user.ID)
	if err == nil {
		remaining := x.bans.GetRemainingDuration(expiresAt)
		formattedExpiry := x.bans.FormatBanExpiry(expiresAt)
		formattedRemaining := x.bans.FormatRemainingTime(remaining)
		
		log.W("User is banned", "user_id", user.ID, "expires_at", expiresAt, "reason", ban.Reason)
		banMsg := x.localization.LocalizeByTd(msg, "MsgBanActive", map[string]interface{}{
			"ExpiresAt": formattedExpiry,
			"Reason":    ban.Reason,
			"Remaining": formattedRemaining,
		})
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, banMsg))
		return
	}

	if msg.Photo != nil && len(msg.Photo) > 0 {
		x.XiCommandPhoto(log, user, msg)
		return
	}

	x.XiCommandText(log, msg)
}

func (x *TelegramHandler) HandleModeCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		if !x.rights.IsUserHasRight(log, user, "switch_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeNoAccess")))
			return
		}
		x.ModeCommandSwitch(log, user, msg)
		return
	}

	var cmd ModeCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeHelpText")))
		return
	}

	switch ctx.Command() {
	case "add <chatid> <type> <name> <config>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")))
			return
		}
		x.ModeCommandAdd(log, user, msg, int64(cmd.Add.ChatID), cmd.Add.Type, cmd.Add.Name, cmd.Add.Config)
	case "list <chatid>":
		if !x.rights.IsUserHasRight(log, user, "switch_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeNoAccess")))
			return
		}
		x.ModeCommandList(log, msg, int64(cmd.List.ChatID))
	case "disable <chatid> <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")))
			return
		}
		x.ModeCommandDisable(log, msg, int64(cmd.Disable.ChatID), cmd.Disable.Type)
	case "enable <chatid> <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")))
			return
		}
		x.ModeCommandEnable(log, msg, int64(cmd.Enable.ChatID), cmd.Enable.Type)
	case "delete <chatid> <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")))
			return
		}
		x.ModeCommandDelete(log, msg, int64(cmd.Delete.ChatID), cmd.Delete.Type)
	case "edit <chatid> <type> <config>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")))
			return
		}
		x.ModeCommandEdit(log, msg, int64(cmd.Edit.ChatID), cmd.Edit.Type, cmd.Edit.Config)
	default:
		log.W("Unknown mode subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeHelpText")))
	}
}

func (x *TelegramHandler) HandleUsersCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgUsersNoAccess")))
		return
	}

	var cmd UsersCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgUsersHelpText")))
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
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgUsersUnknownCommand")))
	}
}

func (x *TelegramHandler) HandleDonationsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	var cmd DonationsCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgDonationsHelpText")))
		return
	}

	switch ctx.Command() {
	case "add <username> <sum>":
		if !x.rights.IsUserHasRight(log, user, "manage_users") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgDonationsNoAccess")))
			return
		}
		x.DonationsCommandAdd(log, msg, cmd.Add.Username, cmd.Add.Sum)
	case "list":
		x.DonationsCommandList(log, msg)
	default:
		log.W("Unknown donations subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgDonationsUnknownCommand")))
	}
}

func (x *TelegramHandler) HandleThisCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.ThisCommand(log, user, msg)
}

func (x *TelegramHandler) HandleStatsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.StatsCommand(log, user, msg)
}

func (x *TelegramHandler) HandleRestartCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if user.Username != nil && *user.Username == "mairwunnx" {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgRestartText"))
		os.Exit(0)
	}
}

func (x *TelegramHandler) HandleStartCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgStartText"))
}

func (x *TelegramHandler) HandlePersonalizationCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationHelpText")))
		return
	}

	var cmd PersonalizationCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationHelpText")))
		return
	}

	switch ctx.Command() {
	case "set <prompt>":
		x.PersonalizationCommandSet(log, user, msg, cmd.Set.Prompt)
	case "remove":
		x.PersonalizationCommandRemove(log, user, msg)
	case "print":
		x.PersonalizationCommandPrint(log, user, msg)
	case "help":
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationHelpText")))
	default:
		log.W("Unknown personalization subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationHelpText")))
	}
}

func (x *TelegramHandler) HandleHelpCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgHelpText"))
}

func (x *TelegramHandler) HandleHealthCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.HealthCommand(log, user, msg)
}

func (x *TelegramHandler) HandleContextCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextHelpText")))
		return
	}

	var cmd ContextCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextHelpText")))
		return
	}

	switch ctx.Command() {
	case "refresh":
		if msg.Chat.Type != "private" && !x.rights.IsUserHasRight(log, user, "manage_context") {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextNoAccess")))
			return
		}
		x.ContextCommandRefresh(log, user, msg)
	case "help":
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextHelpText")))
	default:
		log.W("Unknown context subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextHelpText")))
	}
}

func (x *TelegramHandler) HandleBanCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgUsersNoAccess")))
		return
	}

	var cmd BanCmd
	_, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgBanHelpText")))
		return
	}

	x.BanCommandApply(log, msg, cmd.Username, cmd.Reason, cmd.Duration)
}

func (x *TelegramHandler) HandlePardonCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgUsersNoAccess")))
		return
	}

	var cmd PardonCmd
	_, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPardonHelpText")))
		return
	}

	x.PardonCommandApply(log, msg, cmd.Username)
}