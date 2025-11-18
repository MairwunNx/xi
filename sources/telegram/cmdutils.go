package telegram

import (
	"strings"
	"ximanager/sources/texting/command"
	"ximanager/sources/tracing"

	"errors"

	"github.com/alecthomas/kong"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (x *TelegramHandler) GetRequestText(msg *tgbotapi.Message) string {
	if msg.IsCommand() {
		req := msg.CommandArguments()
		if !msg.Chat.IsPrivate() && len(strings.TrimSpace(req)) == 0 {
			return ""
		}
		return req
	}
	return msg.Text
}

func (x *TelegramHandler) ParseCmd(cmd interface{}, args string) (*kong.Context, error) {
	parser, err := kong.New(cmd)
	if err != nil {
		return nil, err
	}
	return parser.Parse(command.ParseArguments(args))
}

func (x *TelegramHandler) ParseKongCommand(log *tracing.Logger, msg *tgbotapi.Message, cmd interface{}) (*kong.Context, error) {
	args := msg.CommandArguments()
	if args == "" {
		return nil, errors.New("command arguments are empty")
	}

	ctx, err := x.ParseCmd(cmd, args)
	if err != nil {
		log.W("Error parsing command", tracing.InnerError, err)
		return nil, err
	}
	return ctx, nil
}

func (x *TelegramHandler) ParseBooleanArgument(action string) bool {
	switch strings.ToLower(action) {
	case "enable", "true", "1":
		return true
	case "disable", "false", "0":
		return false
	default:
		return false
	}
}