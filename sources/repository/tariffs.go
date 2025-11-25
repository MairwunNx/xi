package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/shopspring/decimal"
)

type ModelMeta struct {
	Name            string `json:"name"`
	AAI             int    `json:"aai"`
	InputPricePerM  string `json:"input_price_per_m"`
	OutputPricePerM string `json:"output_price_per_m"`
	CtxTokens       string `json:"ctx_tokens"`
}

var (
	ErrTariffNotFound       = errors.New("tariff not found")
	ErrTariffKeyEmpty       = errors.New("tariff key cannot be empty")
	ErrTariffKeyTooLong     = errors.New("tariff key cannot exceed 50 characters")
	ErrTariffNameEmpty      = errors.New("tariff display name cannot be empty")
	ErrTariffNameTooLong    = errors.New("tariff display name cannot exceed 100 characters")
	ErrTariffInvalidEffort  = errors.New("invalid reasoning effort (must be: low, medium, high)")
	ErrTariffInvalidLimit   = errors.New("limit values must be non-negative")
	ErrTariffModelsEmpty    = errors.New("dialer_models cannot be empty")
	ErrTariffModelNameEmpty = errors.New("model name cannot be empty")
)

type TariffsRepository struct{}

func NewTariffsRepository() *TariffsRepository {
	return &TariffsRepository{}
}

func (x *TariffsRepository) GetLatestByKey(log *tracing.Logger, key string) (*entities.Tariff, error) {
	defer tracing.ProfilePoint(log, "Tariffs get latest by key completed", "repository.tariffs.get.latest.by.key", "key", key)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	t := query.Q.Tariff
	tariff, err := t.WithContext(ctx).
		Where(t.Key.Eq(key)).
		Order(t.CreatedAt.Desc()).
		First()

	if err != nil {
		return nil, fmt.Errorf("failed to get latest tariff for key %s: %w", key, err)
	}

	return tariff, nil
}

func (x *TariffsRepository) GetAllLatest(log *tracing.Logger) ([]*entities.Tariff, error) {
	defer tracing.ProfilePoint(log, "Tariffs get all latest completed", "repository.tariffs.get.all.latest")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	t := query.Q.Tariff

	// Get distinct keys with their latest tariff
	tariffs, err := t.WithContext(ctx).
		Order(t.Key, t.CreatedAt.Desc()).
		Find()

	if err != nil {
		return nil, fmt.Errorf("failed to get all tariffs: %w", err)
	}

	// Filter to get only the latest for each key
	latestMap := make(map[string]*entities.Tariff)
	for _, tariff := range tariffs {
		if _, exists := latestMap[tariff.Key]; !exists {
			latestMap[tariff.Key] = tariff
		}
	}

	result := make([]*entities.Tariff, 0, len(latestMap))
	for _, tariff := range latestMap {
		result = append(result, tariff)
	}

	return result, nil
}

type TariffConfig struct {
	DisplayName string `json:"display_name"`

	DialerModels          []ModelMeta `json:"dialer_models"`
	DialerReasoningEffort string      `json:"dialer_reasoning_effort"`

	ContextTTLSeconds  int `json:"context_ttl_seconds"`
	ContextMaxMessages int `json:"context_max_messages"`
	ContextMaxTokens   int `json:"context_max_tokens"`

	UsageVisionDaily    int `json:"usage_vision_daily"`
	UsageVisionMonthly  int `json:"usage_vision_monthly"`
	UsageDialerDaily    int `json:"usage_dialer_daily"`
	UsageDialerMonthly  int `json:"usage_dialer_monthly"`
	UsageWhisperDaily   int `json:"usage_whisper_daily"`
	UsageWhisperMonthly int `json:"usage_whisper_monthly"`

	SpendingDailyLimit   string `json:"spending_daily_limit"`
	SpendingMonthlyLimit string `json:"spending_monthly_limit"`
}

func (x *TariffsRepository) CreateTariff(log *tracing.Logger, key string, config *TariffConfig) (*entities.Tariff, error) {
	defer tracing.ProfilePoint(log, "Tariffs create tariff completed", "repository.tariffs.create", "key", key)()

	// Validate key
	if key == "" {
		return nil, ErrTariffKeyEmpty
	}
	if len(key) > 50 {
		return nil, ErrTariffKeyTooLong
	}

	// Validate display name
	if config.DisplayName == "" {
		return nil, ErrTariffNameEmpty
	}
	if len(config.DisplayName) > 100 {
		return nil, ErrTariffNameTooLong
	}

	// Validate reasoning effort
	validEfforts := []string{"low", "medium", "high"}
	effortValid := false
	for _, e := range validEfforts {
		if config.DialerReasoningEffort == e {
			effortValid = true
			break
		}
	}
	if !effortValid {
		return nil, ErrTariffInvalidEffort
	}

	// Validate dialer models
	if len(config.DialerModels) == 0 {
		return nil, ErrTariffModelsEmpty
	}
	for _, model := range config.DialerModels {
		if model.Name == "" {
			return nil, ErrTariffModelNameEmpty
		}
	}

	// Validate non-negative limits
	if config.ContextTTLSeconds < 0 || config.ContextMaxMessages < 0 || config.ContextMaxTokens < 0 ||
		config.UsageVisionDaily < 0 || config.UsageVisionMonthly < 0 ||
		config.UsageDialerDaily < 0 || config.UsageDialerMonthly < 0 ||
		config.UsageWhisperDaily < 0 || config.UsageWhisperMonthly < 0 {
		return nil, ErrTariffInvalidLimit
	}

	// Parse spending limits
	dailyLimit, err := decimal.NewFromString(config.SpendingDailyLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid spending_daily_limit: %w", err)
	}
	if dailyLimit.IsNegative() {
		return nil, ErrTariffInvalidLimit
	}

	monthlyLimit, err := decimal.NewFromString(config.SpendingMonthlyLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid spending_monthly_limit: %w", err)
	}
	if monthlyLimit.IsNegative() {
		return nil, ErrTariffInvalidLimit
	}

	// Serialize dialer models to JSON
	modelsJSON, err := serializeDialerModels(config.DialerModels)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize dialer_models: %w", err)
	}

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	tariff := &entities.Tariff{
		Key:                   key,
		DisplayName:           config.DisplayName,
		DialerModels:          modelsJSON,
		DialerReasoningEffort: config.DialerReasoningEffort,
		ContextTTLSeconds:     config.ContextTTLSeconds,
		ContextMaxMessages:    config.ContextMaxMessages,
		ContextMaxTokens:      config.ContextMaxTokens,
		UsageVisionDaily:      config.UsageVisionDaily,
		UsageVisionMonthly:    config.UsageVisionMonthly,
		UsageDialerDaily:      config.UsageDialerDaily,
		UsageDialerMonthly:    config.UsageDialerMonthly,
		UsageWhisperDaily:     config.UsageWhisperDaily,
		UsageWhisperMonthly:   config.UsageWhisperMonthly,
		SpendingDailyLimit:    dailyLimit,
		SpendingMonthlyLimit:  monthlyLimit,
	}

	t := query.Q.Tariff
	err = t.WithContext(ctx).Create(tariff)
	if err != nil {
		log.E("Failed to create tariff", tracing.InnerError, err)
		return nil, err
	}

	log.I("Created tariff", "key", key, "id", tariff.ID)
	return tariff, nil
}

func serializeDialerModels(models []ModelMeta) ([]byte, error) {
	if models == nil {
		models = []ModelMeta{}
	}
	return json.Marshal(models)
}