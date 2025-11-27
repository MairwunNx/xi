package telegram

import (
	"os"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/repository"
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
		formattedExpiry := x.bans.FormatBanExpiry(msg, expiresAt)
		formattedRemaining := x.bans.FormatRemainingTime(msg, remaining)

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
		x.ModeCommandShowList(log, user, msg)
		return
	}

	if args == "help" {
		helpMsg := x.localization.LocalizeBy(msg, "MsgModeHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	var cmd ModeCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		helpMsg := x.localization.LocalizeBy(msg, "MsgModeHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	switch ctx.Command() {
	case "create":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			noAccessMsg := x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
			return
		}
		x.ModeCommandCreateStart(log, user, msg)
	case "edit <type>":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			noAccessMsg := x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
			return
		}
		x.ModeCommandEditStart(log, user, msg, cmd.Edit.Type)
	case "info":
		if !x.rights.IsUserHasRight(log, user, "edit_mode") {
			noAccessMsg := x.localization.LocalizeBy(msg, "MsgModeModifyNoAccess")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
			return
		}
		x.ModeCommandInfoList(log, user, msg)
	default:
		log.W("Unknown mode subcommand", tracing.InternalCommand, ctx.Command())
		helpMsg := x.localization.LocalizeBy(msg, "MsgModeHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
	}
}

func (x *TelegramHandler) HandleCancelCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	state, err := x.chatState.GetState(log, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.E("Failed to get chat state", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgCancelNothingToCancel"))
		return
	}

	if state == nil || state.Status == repository.ChatStateNone {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgCancelNothingToCancel"))
		return
	}

	err = x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.E("Failed to clear chat state", tracing.InnerError, err)
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgCancelSuccess"))
}

func (x *TelegramHandler) HandleUsersCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		noAccessMsg := x.localization.LocalizeBy(msg, "MsgUsersNoAccess")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
		return
	}

	var cmd UsersCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		helpMsg := x.localization.LocalizeBy(msg, "MsgUsersHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
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
		unknownCmdMsg := x.localization.LocalizeBy(msg, "MsgUsersUnknownCommand")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, unknownCmdMsg))
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
	helpMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationHelpText")

	args := msg.CommandArguments()
	if args == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	var cmd PersonalizationCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
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
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
	default:
		log.W("Unknown personalization subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
	}
}

func (x *TelegramHandler) HandleHelpCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgHelpText"))
}

func (x *TelegramHandler) HandleHealthCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.HealthCommand(log, user, msg)
}

func (x *TelegramHandler) HandleContextCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	helpMsg := x.localization.LocalizeBy(msg, "MsgContextHelpText")

	args := msg.CommandArguments()
	if args == "" {
		// Show context info with action buttons when no arguments
		x.ContextCommandInfo(log, user, msg)
		return
	}

	var cmd ContextCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	switch ctx.Command() {
	case "help":
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
	default:
		log.W("Unknown context subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
	}
}

func (x *TelegramHandler) HandleBanCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		noAccessMsg := x.localization.LocalizeBy(msg, "MsgUsersNoAccess")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
		return
	}

	var cmd BanCmd
	_, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		helpMsg := x.localization.LocalizeBy(msg, "MsgBanHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	x.BanCommandApply(log, msg, cmd.Username, cmd.Reason, cmd.Duration)
}

func (x *TelegramHandler) HandlePardonCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		noAccessMsg := x.localization.LocalizeBy(msg, "MsgUsersNoAccess")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
		return
	}

	var cmd PardonCmd
	_, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		helpMsg := x.localization.LocalizeBy(msg, "MsgPardonHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	x.PardonCommandApply(log, msg, cmd.Username)
}

func (x *TelegramHandler) HandleBroadcastCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "broadcast") {
		noAccessMsg := x.localization.LocalizeBy(msg, "MsgBroadcastNoAccess")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
		return
	}

	var cmd BroadcastCmd
	_, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		helpMsg := x.localization.LocalizeBy(msg, "MsgBroadcastHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	x.BroadcastCommandApply(log, user, msg, cmd.Text)
}

func (x *TelegramHandler) HandleTariffCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	if !x.rights.IsUserHasRight(log, user, "manage_tariffs") {
		noAccessMsg := x.localization.LocalizeBy(msg, "MsgTariffNoAccess")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noAccessMsg))
		return
	}

	helpMsg := x.localization.LocalizeBy(msg, "MsgTariffHelpText")

	args := msg.CommandArguments()
	if args == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	var cmd TariffCmd
	ctx, err := x.ParseKongCommand(log, msg, &cmd)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	switch ctx.Command() {
	case "add <key> <config>":
		x.TariffCommandAdd(log, msg, cmd.Add.Key, cmd.Add.Config)
	case "list":
		x.TariffCommandList(log, msg)
	case "get <key>":
		x.TariffCommandGet(log, msg, cmd.Get.Key)
	default:
		log.W("Unknown tariff subcommand", tracing.InternalCommand, ctx.Command())
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
	}
}