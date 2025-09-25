package telegram

import (
	"context"
	"sync"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Poller struct {
	bot      *tgbotapi.BotAPI
	log      *tracing.Logger
	config   *PollerConfig
	diplomat *Diplomat
	handler  *TelegramHandler
	
	semaphore chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewPoller(bot *tgbotapi.BotAPI, log *tracing.Logger, diplomat *Diplomat, config *PollerConfig, handler *TelegramHandler) *Poller {
	ctx, cancel := context.WithCancel(context.Background())
	return &Poller{
		bot:       bot,
		log:       log,
		diplomat:  diplomat,
		config:    config,
		handler:   handler,
		semaphore: make(chan struct{}, config.MaxConcurrency),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (x *Poller) Start() {
	update := tgbotapi.NewUpdate(0)
	update.Timeout = x.config.Timeout
	update.AllowedUpdates = x.config.AllowedUpdates

	x.log.I("Starting poller with max concurrency", "maxConcurrency", x.config.MaxConcurrency)

	for update := range x.bot.GetUpdatesChan(update) {
		if msg := update.Message; msg != nil {
			select {
			case <-x.ctx.Done():
				x.log.I("Context cancelled, stopping message processing")
				return
			default:
			}

			x.wg.Add(1)
			go x.handleMessageConcurrently(update)
		}
	}
}

func (x *Poller) handleMessageConcurrently(update tgbotapi.Update) {
	defer x.wg.Done()

	select {
	case x.semaphore <- struct{}{}:
		defer func() { <-x.semaphore }()
	case <-x.ctx.Done():
		return
	}

	msg := update.Message
	user := update.SentFrom()

	log := x.log.With(
		tracing.UserId, user.ID,
		tracing.UserName, user.UserName,
		tracing.ChatType, msg.Chat.Type,
		tracing.ChatId, msg.Chat.ID,
		tracing.MessageId, msg.MessageID,
		tracing.MessageDate, msg.Date,
	)

	if err := x.handler.HandleMessage(log, msg); err != nil {
		x.diplomat.Reply(log, msg, texting.MsgXiError)
	}

	log.I("Message handled")
}

func (x *Poller) Stop() {
	x.log.I("Stopping poller...")
	x.cancel()
	x.bot.StopReceivingUpdates()
	x.wg.Wait()
	x.log.I("Poller stopped")
}