package telegram

import (
	"sync"
	"time"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TypingManager struct {
	bot     *tgbotapi.BotAPI
	active  map[int64]chan struct{}
	mu      sync.Mutex
	log     *tracing.Logger
}

func NewTypingManager(bot *tgbotapi.BotAPI, log *tracing.Logger) *TypingManager {
	return &TypingManager{
		bot:    bot,
		active: make(map[int64]chan struct{}),
		log:    log,
	}
}

func (tm *TypingManager) Start(chatID int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.active[chatID]; exists {
		return
	}

	stopCh := make(chan struct{})
	tm.active[chatID] = stopCh

	go tm.typingLoop(chatID, stopCh)
}

func (tm *TypingManager) Stop(chatID int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if stopCh, exists := tm.active[chatID]; exists {
		close(stopCh)
		delete(tm.active, chatID)
	}
}

func (tm *TypingManager) typingLoop(chatID int64, stopCh chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	tm.sendTyping(chatID)

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			tm.sendTyping(chatID)
		}
	}
}

func (tm *TypingManager) sendTyping(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	if _, err := tm.bot.Send(action); err != nil {
		tm.log.W("Failed to send typing action", tracing.InnerError, err, "chat_id", chatID)
	}
}