package entities

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	TariffKeyBronze = "bronze"
	TariffKeySilver = "silver"
	TariffKeyGold   = "gold"
)

type Tariff struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Key         string    `gorm:"column:key;not null;index:idx_key_created,priority:1"`
	DisplayName string    `gorm:"column:display_name;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;default:now();index:idx_key_created,priority:2,sort:desc"`

	DialerModels          []byte   `gorm:"column:dialer_models;type:jsonb;not null"`
	DialerReasoningEffort string   `gorm:"column:dialer_reasoning_effort;not null"`
	VisionPrimaryModel    string   `gorm:"column:vision_primary_model;not null"`
	VisionFallbackModels  []string `gorm:"column:vision_fallback_models;type:text[];not null;serializer:json"`

	ContextTTLSeconds  int `gorm:"column:context_ttl_seconds;not null"`
	ContextMaxMessages int `gorm:"column:context_max_messages;not null"`
	ContextMaxTokens   int `gorm:"column:context_max_tokens;not null"`

	UsageVisionDaily    int `gorm:"column:usage_vision_daily;not null"`
	UsageVisionMonthly  int `gorm:"column:usage_vision_monthly;not null"`
	UsageDialerDaily    int `gorm:"column:usage_dialer_daily;not null"`
	UsageDialerMonthly  int `gorm:"column:usage_dialer_monthly;not null"`
	UsageWhisperDaily   int `gorm:"column:usage_whisper_daily;not null"`
	UsageWhisperMonthly int `gorm:"column:usage_whisper_monthly;not null"`

	SpendingDailyLimit   decimal.Decimal `gorm:"column:spending_daily_limit;type:decimal(10,2);not null"`
	SpendingMonthlyLimit decimal.Decimal `gorm:"column:spending_monthly_limit;type:decimal(10,2);not null"`
}

func (Tariff) TableName() string {
	return "xi_tariffs"
}