package telegram

import (
	"context"
	"ximanager/sources/tracing"

	"go.uber.org/fx"
)

var Module = fx.Module("telegram",
	fx.Provide(
		NewBotAPI,
		NewDiplomat,
		NewTelegramHandler,
		NewPoller,
	),

	fx.Invoke(func(lc fx.Lifecycle, poller *Poller, log *tracing.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				go poller.Start()
				log.I("Telegram poller started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				poller.Stop()
				log.I("Telegram poller stopped")
				return nil
			},
		})
	}),
)