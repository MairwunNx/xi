package telegram

import (
	"context"
	"sync"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/localization"
	"ximanager/sources/metrics"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type chatQueue struct {
	messages chan tgbotapi.Update
	lastUsed time.Time
}

type Poller struct {
	bot          *tgbotapi.BotAPI
	log          *tracing.Logger
	config       *configuration.Config
	diplomat     *Diplomat
	handler      *TelegramHandler
	localization *localization.LocalizationManager
	metrics      *metrics.MetricsService

	chatQueues map[int64]*chatQueue
	queuesMux  sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewPoller(bot *tgbotapi.BotAPI, log *tracing.Logger, diplomat *Diplomat, config *configuration.Config, handler *TelegramHandler, localization *localization.LocalizationManager, metrics *metrics.MetricsService) *Poller {
	ctx, cancel := context.WithCancel(context.Background())
	poller := &Poller{
		bot:          bot,
		log:          log,
		diplomat:     diplomat,
		config:       config,
		handler:      handler,
		localization: localization,
		metrics:      metrics,
		chatQueues:   make(map[int64]*chatQueue),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	go poller.cleanupInactiveQueues()
	return poller
}

func (x *Poller) Start() {
	update := tgbotapi.NewUpdate(0)
	update.Timeout = x.config.Telegram.PollerTimeout
	update.AllowedUpdates = x.config.Telegram.AllowedUpdates

	x.log.I("Starting poller with per-chat sequential processing")

	for update := range x.bot.GetUpdatesChan(update) {
		if msg := update.Message; msg != nil {
			select {
			case <-x.ctx.Done():
				x.log.I("Context cancelled, stopping message processing")
				return
			default:
			}

			chatID := msg.Chat.ID
			x.enqueueMessage(chatID, update)
		} else if cb := update.CallbackQuery; cb != nil {
			select {
			case <-x.ctx.Done():
				x.log.I("Context cancelled, stopping callback processing")
				return
			default:
			}

			if cb.Message != nil {
				chatID := cb.Message.Chat.ID
				x.enqueueMessage(chatID, update)
			}
		}
	}
}

func (x *Poller) enqueueMessage(chatID int64, update tgbotapi.Update) {
	queue := x.getOrCreateQueue(chatID)
	
	select {
	case queue.messages <- update:
		// Сообщение добавлено в очередь успешно
	case <-x.ctx.Done():
		// Контекст отменен
		return
	}
}

func (x *Poller) getOrCreateQueue(chatID int64) *chatQueue {
	x.queuesMux.RLock()
	queue, exists := x.chatQueues[chatID]
	x.queuesMux.RUnlock()
	
	if exists {
		queue.lastUsed = time.Now()
		return queue
	}
	
	x.queuesMux.Lock()
	defer x.queuesMux.Unlock()
	
	if queue, exists = x.chatQueues[chatID]; exists {
		queue.lastUsed = time.Now()
		return queue
	}
	
	queue = &chatQueue{
		messages: make(chan tgbotapi.Update, 100), // Buffer for smooth processing
		lastUsed: time.Now(),
	}
	x.chatQueues[chatID] = queue
	
	x.wg.Add(1)
	go x.processChatQueue(chatID, queue)
	
	x.log.D("Created new queue for chat", "chatId", chatID)
	return queue
}

func (x *Poller) processChatQueue(chatID int64, queue *chatQueue) {
	defer x.wg.Done()
	defer x.log.D("Chat queue processor stopped", "chatId", chatID)
	
	for {
		select {
		case <-x.ctx.Done():
			return
		case update := <-queue.messages:
			queue.lastUsed = time.Now()
			x.handleUpdate(update)
		}
	}
}

func (x *Poller) handleUpdate(update tgbotapi.Update) {
	user := update.SentFrom()
	var chatID int64
	var msgID int
	var msgDate int
	var chatType string

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		msgID = update.Message.MessageID
		msgDate = update.Message.Date
		chatType = update.Message.Chat.Type
	} else if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
		chatID = update.CallbackQuery.Message.Chat.ID
		msgID = update.CallbackQuery.Message.MessageID
		msgDate = update.CallbackQuery.Message.Date
		chatType = update.CallbackQuery.Message.Chat.Type
	}

	log := x.log.With(
		tracing.UserId, user.ID,
		tracing.UserName, user.UserName,
		tracing.ChatType, chatType,
		tracing.ChatId, chatID,
		tracing.MessageId, msgID,
		tracing.MessageDate, msgDate,
	)

	start := time.Now()

	if update.Message != nil {
		if err := x.handler.HandleMessage(log, update.Message); err != nil {
			errorMsg := x.localization.LocalizeBy(update.Message, "MsgXiError")
			x.diplomat.Reply(log, update.Message, errorMsg)
			x.metrics.RecordMessageHandled("error")
		} else {
			x.metrics.RecordMessageHandled("success")
		}
	} else if update.CallbackQuery != nil {
		if err := x.handler.HandleCallback(log, update.CallbackQuery); err != nil {
			x.log.E("Error handling callback", tracing.InnerError, err)
		}
	}

	x.metrics.RecordMessageProcessingDuration(time.Since(start))

	log.D("Update handled")
}

func (x *Poller) cleanupInactiveQueues() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-x.ctx.Done():
			return
		case <-ticker.C:
			x.performCleanup()
		}
	}
}

func (x *Poller) performCleanup() {
	x.queuesMux.Lock()
	defer x.queuesMux.Unlock()
	
	threshold := time.Now().Add(-30 * time.Minute)
	removed := 0
	
	for chatID, queue := range x.chatQueues {
		if queue.lastUsed.Before(threshold) && len(queue.messages) == 0 {
			close(queue.messages)
			delete(x.chatQueues, chatID)
			removed++
		}
	}
	
	if removed > 0 {
		x.log.D("Cleaned up inactive chat queues", "removed", removed, "remaining", len(x.chatQueues))
	}
}

func (x *Poller) Stop() {
	x.log.I("Stopping poller...")
	x.cancel()
	x.bot.StopReceivingUpdates()
	
	x.queuesMux.Lock()
	for chatID, queue := range x.chatQueues {
		close(queue.messages)
		x.log.D("Closed queue for chat", "chatId", chatID)
	}
	x.queuesMux.Unlock()
	
	x.wg.Wait()
	x.log.I("Poller stopped")
}