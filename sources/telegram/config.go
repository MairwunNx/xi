package telegram

import (
	"fmt"
	"ximanager/sources/platform"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotConfig struct {
	Token string
}

type DiplomatConfig struct {
	ChunkSize int
}

type PollerConfig struct {
	Timeout           int
	AllowedUpdates    []string
	MaxConcurrency    int
}

func (c *BotConfig) Validate() error {
	return platform.ValidateTelegramBotToken(c.Token)
}

func NewBotConfig() *BotConfig {
	config := &BotConfig{
		Token: platform.Get("TELEGRAM_BOT_TOKEN", ""),
	}
	
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid bot configuration: %v", err))
	}
	
	return config
}

func NewDiplomatConfig() *DiplomatConfig {
	return &DiplomatConfig{
		ChunkSize: platform.GetAsInt("TELEGRAM_DIPLOMAT_CHUNK_SIZE", 4096),
	}
}

func NewPollerConfig() *PollerConfig {
	return &PollerConfig{
		Timeout:        platform.GetAsInt("TELEGRAM_POLLER_TIMEOUT", 120),
		AllowedUpdates: platform.GetAsSlice("TELEGRAM_POLLER_ALLOWED_UPDATES", []string{tgbotapi.UpdateTypeMessage}),
		MaxConcurrency: platform.GetAsInt("TELEGRAM_POLLER_MAX_CONCURRENCY", 10),
	}
}
