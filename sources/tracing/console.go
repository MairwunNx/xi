package tracing

import (
	"context"
	"log/slog"
	"os"
	"time"
)

const (
	ExecutionTime      = "exe_time"
	OutsiderKind       = "outsider_kind"
	ProxyUrl           = "proxy_url"
	ProxyRes           = "proxy_res"
	AiKind             = "ai_kind"
	AiModel            = "ai_model"
	AiAttempt          = "ai_attempt"
	AiBackoff          = "ai_backoff"
	InnerError         = "inner_error"
	UserId             = "user_id"
	UserName           = "user_name"
	InternalUserActive = "internal_user_active"
	InternalUserRights = "internal_user_rights"
	InternalUserWindow = "internal_user_window"
	InternalUserStack  = "internal_user_stack"
	ChatType           = "chat_type"
	ChatId             = "chat_id"
	MessageId          = "message_id"
	MessageDate        = "message_date"
	SqlQuery           = "sql_query"
	AiProvider         = "ai_provider"
	AiTokens           = "ai_tokens"
	AiCost             = "ai_cost"
	ModeId             = "mode_id"
	ModeName           = "mode_name"
	CommandIssued      = "command_issued"
	Scope              = "scope"
	InternalCommand    = "internal_command"
)

type Logger struct {
	log *slog.Logger
	ctx context.Context
}

func NewConsoleLogger() *Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger.InfoContext(ctx, "Initializing  logger")
	return &Logger{log: logger, ctx: ctx}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{log: l.log.With(args...), ctx: l.ctx}
}

func (l *Logger) D(msg string, args ...any) {
	l.log.DebugContext(l.ctx, msg, args...)
}

func (l *Logger) I(msg string, args ...any) {
	l.log.InfoContext(l.ctx, msg, args...)
}

func (l *Logger) W(msg string, args ...any) {
	l.log.WarnContext(l.ctx, msg, args...)
}

func (l *Logger) E(msg string, args ...any) {
	l.log.ErrorContext(l.ctx, msg, args...)
}

func (l *Logger) F(msg string, args ...any) {
	l.log.ErrorContext(l.ctx, msg, args...)
	panic(msg)
}