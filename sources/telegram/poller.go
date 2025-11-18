package telegram

import (
	"context"
	"sync"
	"time"
	"ximanager/sources/localization"
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
	config       *PollerConfig
	diplomat     *Diplomat
	handler      *TelegramHandler
	localization *localization.LocalizationManager

	chatQueues map[int64]*chatQueue
	queuesMux  sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewPoller(bot *tgbotapi.BotAPI, log *tracing.Logger, diplomat *Diplomat, config *PollerConfig, handler *TelegramHandler, localization *localization.LocalizationManager) *Poller {
	ctx, cancel := context.WithCancel(context.Background())
	poller := &Poller{
		bot:          bot,
		log:          log,
		diplomat:     diplomat,
		config:       config,
		handler:      handler,
		localization: localization,
		chatQueues:   make(map[int64]*chatQueue),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	go poller.cleanupInactiveQueues()
	return poller
}

func (x *Poller) Start() {
	update := tgbotapi.NewUpdate(0)
	update.Timeout = x.config.Timeout
	update.AllowedUpdates = x.config.AllowedUpdates

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
			x.handleMessage(update)
		}
	}
}

func (x *Poller) handleMessage(update tgbotapi.Update) {
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
		errorMsg := x.localization.LocalizeBy(msg, "MsgXiError")
		x.diplomat.Reply(log, msg, errorMsg)
	}

	log.D("Message handled")
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