package repository

import (
	"context"
	"errors"
	"fmt"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/shopspring/decimal"
)

var (
	ErrTariffNotFound      = errors.New("tariff not found")
	ErrTariffKeyEmpty      = errors.New("tariff key cannot be empty")
	ErrTariffKeyTooLong    = errors.New("tariff key cannot exceed 50 characters")
	ErrTariffNameEmpty     = errors.New("tariff display name cannot be empty")
	ErrTariffNameTooLong   = errors.New("tariff display name cannot exceed 100 characters")
	ErrTariffInvalidLimit  = errors.New("limit values must be non-negative")
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

	RequestsPerDay   int   `json:"requests_per_day"`
	RequestsPerMonth int   `json:"requests_per_month"`
	TokensPerDay     int64 `json:"tokens_per_day"`
	TokensPerMonth   int64 `json:"tokens_per_month"`

	SpendingDailyLimit   string `json:"spending_daily_limit"`
	SpendingMonthlyLimit string `json:"spending_monthly_limit"`

	Price int `json:"price"`
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

	// Validate non-negative limits
	if config.RequestsPerDay < 0 || config.RequestsPerMonth < 0 ||
		config.TokensPerDay < 0 || config.TokensPerMonth < 0 || config.Price < 0 {
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

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	tariff := &entities.Tariff{
		Key:                  key,
		DisplayName:          config.DisplayName,
		RequestsPerDay:       config.RequestsPerDay,
		RequestsPerMonth:     config.RequestsPerMonth,
		TokensPerDay:         config.TokensPerDay,
		TokensPerMonth:       config.TokensPerMonth,
		SpendingDailyLimit:   dailyLimit,
		SpendingMonthlyLimit: monthlyLimit,
		Price:                config.Price,
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
