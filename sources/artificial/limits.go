package artificial

import (
	"context"
	"fmt"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type LimitType string

const (
	LimitTypeDaily   LimitType = "daily"
	LimitTypeMonthly LimitType = "monthly"
)

type SpendingLimitExceededError struct {
	UserGrade    platform.UserGrade
	LimitType    LimitType
	LimitAmount  decimal.Decimal
	CurrentSpend decimal.Decimal
}

func (e *SpendingLimitExceededError) Error() string {
	return fmt.Sprintf("spending limit exceeded for %s user: %s limit is %s, but current spend is %s",
		e.UserGrade, e.LimitType, e.LimitAmount.String(), e.CurrentSpend.String())
}

type SpendingLimiter struct {
	redis     *redis.Client
	config    *configuration.Config
	usage     *repository.UsageRepository
	donations *repository.DonationsRepository
	tariffs   *repository.TariffsRepository
	log       *tracing.Logger
}

func NewSpendingLimiter(
	redis *redis.Client,
	config *configuration.Config,
	usage *repository.UsageRepository,
	donations *repository.DonationsRepository,
	tariffs *repository.TariffsRepository,
	log *tracing.Logger,
) *SpendingLimiter {
	return &SpendingLimiter{
		redis:     redis,
		config:    config,
		usage:     usage,
		donations: donations,
		tariffs:   tariffs,
		log:       log,
	}
}

func (x *SpendingLimiter) CheckSpendingLimits(logger *tracing.Logger, user *entities.User) error {
	userGrade, err := x.donations.GetUserGrade(logger, user)
	if err != nil {
		logger.W("Failed to get user grade, using bronze as default", tracing.InnerError, err)
		userGrade = platform.GradeBronze
	}

	tariff, err := getTariffWithFallback(x.log, x.tariffs, userGrade)
	if err != nil {
		return fmt.Errorf("failed to fetch tariff limits: %w", err)
	}

	dailyLimit := tariff.SpendingDailyLimit
	monthlyLimit := tariff.SpendingMonthlyLimit

	dailySpend, err := x.getSpend(logger, user, "daily")
	if err != nil {
		return err
	}
	if dailySpend.GreaterThan(dailyLimit) {
		return &SpendingLimitExceededError{
			UserGrade:    userGrade,
			LimitType:    LimitTypeDaily,
			LimitAmount:  dailyLimit,
			CurrentSpend: dailySpend,
		}
	}

	monthlySpend, err := x.getSpend(logger, user, "monthly")
	if err != nil {
		return err
	}
	if monthlySpend.GreaterThan(monthlyLimit) {
		return &SpendingLimitExceededError{
			UserGrade:    userGrade,
			LimitType:    LimitTypeMonthly,
			LimitAmount:  monthlyLimit,
			CurrentSpend: monthlySpend,
		}
	}

	return nil
}

func (x *SpendingLimiter) getSpend(logger *tracing.Logger, user *entities.User, period string) (decimal.Decimal, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 3*time.Second)
	defer cancel()

	key := x.getSpendKey(user.ID.String(), period)

	// Try to get from Redis first
	cachedSpend, err := x.redis.Get(ctx, key).Result()
	if err == nil {
		spend, parseErr := decimal.NewFromString(cachedSpend)
		if parseErr != nil {
			logger.W("Failed to parse cached spend from Redis, falling back to DB", "key", key, "cached_value", cachedSpend, tracing.InnerError, parseErr)
		} else {
			return spend, nil
		}
	}

	// If not in Redis, get from DB
	var spend decimal.Decimal
	var dbErr error
	now := time.Now()

	switch period {
	case "daily":
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		spend, dbErr = x.usage.GetUserCostSince(logger, user, startOfDay)
	case "monthly":
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		spend, dbErr = x.usage.GetUserCostSince(logger, user, startOfMonth)
	default:
		return decimal.Zero, fmt.Errorf("invalid period: %s", period)
	}

	if dbErr != nil {
		logger.E("Failed to get user spend from DB", "period", period, tracing.InnerError, dbErr)
		return decimal.Zero, dbErr
	}

	// Cache in Redis
	ttl := 24 * time.Hour
	if period == "monthly" {
		ttl = 31 * 24 * time.Hour
	}
	err = x.redis.Set(ctx, key, spend.String(), ttl).Err()
	if err != nil {
		logger.W("Failed to cache spend in Redis", "key", key, tracing.InnerError, err)
	}

	return spend, nil
}

func (x *SpendingLimiter) IncrementSpend(logger *tracing.Logger, userID string, amount decimal.Decimal) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 2*time.Second)
	defer cancel()

	dailyKey := x.getSpendKey(userID, "daily")
	monthlyKey := x.getSpendKey(userID, "monthly")

	pipe := x.redis.TxPipeline()
	pipe.IncrByFloat(ctx, dailyKey, amount.InexactFloat64())
	pipe.IncrByFloat(ctx, monthlyKey, amount.InexactFloat64())
	pipe.Expire(ctx, dailyKey, 25*time.Hour)
	pipe.Expire(ctx, monthlyKey, 32*24*time.Hour)

	if _, err := pipe.Exec(ctx); err != nil {
		logger.E("Failed to increment spend in Redis", "user_id", userID, "amount", amount.String(), tracing.InnerError, err)
	}
}

func (x *SpendingLimiter) getSpendKey(userID string, period string) string {
	now := time.Now()
	var timePart string
	switch period {
	case "daily":
		timePart = now.Format("2006-01-02")
	case "monthly":
		timePart = now.Format("2006-01")
	}
	return fmt.Sprintf("spend:%s:%s:%s", period, userID, timePart)
}

func (x *SpendingLimiter) AddSpend(logger *tracing.Logger, user *entities.User, cost decimal.Decimal) {
	x.IncrementSpend(logger, user.ID.String(), cost)
}
