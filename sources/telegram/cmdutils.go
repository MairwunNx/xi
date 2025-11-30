package telegram

import (
	"errors"
	"strings"
	"ximanager/sources/framework/commands"
	"ximanager/sources/tracing"

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

func (x *TelegramHandler) ParseCommand(log *tracing.Logger, msg *tgbotapi.Message, parser *commands.Parser) (*commands.ParseResult, error) {
	args := msg.CommandArguments()
	if args == "" {
		return nil, errors.New("command arguments are empty")
	}

	result, err := parser.Parse(args)
	if err != nil {
		log.W("Error parsing command", tracing.InnerError, err)
		return nil, err
	}
	return result, nil
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